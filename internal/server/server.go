package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"net"
	"os"
	"os_coursach/internal/server/models"
	"os_coursach/internal/worker"
	"os_coursach/pkg/logger"
	"sync"
	"time"
)

type sets struct {
	host    string
	port    int
	logPath string
}

type Connection struct {
	net.Conn

	InCh  chan []byte
	OutCh chan []byte
}

type Server struct {
	sets sets

	l net.Listener

	cons map[uuid.UUID]Connection
	mu   sync.RWMutex

	reportsChan <-chan string
	lastMsg     []byte

	logger *slog.Logger
}

func New(
	host string,
	port int,
	logPath string,
) *Server {
	s := Server{
		sets: sets{
			host:    host,
			port:    port,
			logPath: logPath,
		},
		cons: make(map[uuid.UUID]Connection),
	}

	return &s
}

func (s *Server) RunWithWorker(
	ctx context.Context,
	w worker.Worker,
	serverName string,
) error {
	listener, err := net.Listen(
		"tcp",
		fmt.Sprintf("%s:%d", s.sets.host, s.sets.port),
	)

	if err != nil {
		return fmt.Errorf("start listener: %w", err)
	}

	s.logger = logger.NewLogger(s.sets.logPath, logger.WithField("server_name", serverName))

	s.l = listener
	s.reportsChan = w.Start(ctx)

	go s.connectionLoop(ctx)

	go s.handleUpdates(ctx)

	s.logger.LogAttrs(ctx, slog.LevelInfo, "server started")

	return nil
}

func (s *Server) handleUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("server stopped")
			return
		case reportMsg := <-s.reportsChan:
			err := s.broadcast(ctx, reportMsg)
			if err != nil {
				s.logger.LogAttrs(
					ctx, slog.LevelError, "broadcast",
					slog.String("error", err.Error()),
				)
			}
		}
	}
}

func (s *Server) connectionLoop(ctx context.Context) {
	for {
		c, err := s.l.Accept()
		if err != nil {

			if errors.Is(err, net.ErrClosed) {
				s.logger.LogAttrs(
					ctx, slog.LevelWarn, "connection closed on accept",
					slog.String("error", err.Error()),
				)
				continue
			}
			s.logger.LogAttrs(
				ctx, slog.LevelError, "connection accept",
				slog.String("error", err.Error()),
			)

			continue
		}

		// запускаем поток для обработки соединения
		go s.handleConn(ctx, c)

	}
}

func newConnection(conn net.Conn) Connection {
	return Connection{
		Conn:  conn,
		InCh:  make(chan []byte, 10),
		OutCh: make(chan []byte, 10),
	}
}

func (s *Server) addNewConnection(conn Connection) uuid.UUID {
	id := uuid.New()
	s.mu.Lock()
	s.cons[id] = conn
	s.mu.Unlock()

	return id
}

func (s *Server) dropConnection(conID uuid.UUID) {
	s.mu.Lock()
	c, ok := s.cons[conID]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.cons, conID)
	s.mu.Unlock()

	s.logger.Info(
		"connection-removed",
		slog.String("connectionID", conID.String()),
		slog.String("address", c.RemoteAddr().String()),
	)

	c.Conn.Close()

	close(c.InCh)
	close(c.OutCh)
}

func (s *Server) handleConn(
	ctx context.Context,
	conn net.Conn,
) {
	// инициализируем соединение, обогащаем структуру net.Conn каналами
	// для общения между потоками
	connection := newConnection(conn)

	// добавляем соединение в хеш-таблицу сервера
	conID := s.addNewConnection(connection)

	s.logger.LogAttrs(
		ctx, slog.LevelInfo, "new-connection",
		slog.String("connectionID", conID.String()),
		slog.String("address", connection.Conn.RemoteAddr().String()),
	)

	// Закрываем соединение при завершени главного цикла
	defer s.dropConnection(conID)

	connection.InCh <- s.lastMsg

	deadlineTicker := time.NewTicker(time.Second * 10)

	for {
		select {
		// если контекст программы завершился -- выходим из главного цикла
		case <-ctx.Done():
			return
		// если нам пришло сообщение из канала: отправляем его
		case msg := <-connection.InCh:

			// добавляем дедлайн для записи, чтобы избежать висячих соединений
			err := connection.SetWriteDeadline(time.Now().Add(time.Second * 5))
			if err != nil {
				s.logger.LogAttrs(
					ctx, slog.LevelWarn, "set-write-deadline",
					slog.String("connectionID", conID.String()),
					slog.String("error", err.Error()),
					slog.String("address", connection.Conn.RemoteAddr().String()),
				)
			}
			// записываем наше сообщение
			_, err = connection.Write(msg)
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) {
					s.logger.LogAttrs(
						ctx, slog.LevelError, "write-deadline-exceed",
						slog.String("connectionID", conID.String()),
						slog.String("error", err.Error()),
						slog.String("address", connection.Conn.RemoteAddr().String()),
					)
				}
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
					s.logger.LogAttrs(
						ctx, slog.LevelError, "connection-closed",
						slog.String("connectionID", conID.String()),
						slog.String("address", connection.Conn.RemoteAddr().String()),
					)
					return
				}
			}
		case <-deadlineTicker.C:
			// устанавливаем дедлайн для соединения и обновляем его
			err := connection.SetDeadline(time.Now().Add(time.Second * 30))
			if err != nil {
				s.logger.LogAttrs(
					ctx, slog.LevelError, "set-connection-deadline",
					slog.String("connectionID", conID.String()),
					slog.String("address", connection.Conn.RemoteAddr().String()),
					slog.String("error", err.Error()),
				)
			}

		}
	}
}

func (s *Server) broadcast(
	ctx context.Context,
	msg string,
) error {
	broadcastMessage := models.BroadcastMessage{Message: msg, Timestamp: time.Now()}
	raw, err := json.Marshal(&broadcastMessage)
	if err != nil {
		return fmt.Errorf("json unmarshal message: %w", err)
	}

	s.logger.LogAttrs(
		ctx, slog.LevelInfo, "broadcast msg",
		slog.String("timestamp", broadcastMessage.Timestamp.String()),
		slog.String("message", broadcastMessage.Message),
	)

	s.lastMsg = raw

	s.mu.Lock()
	for _, conn := range s.cons {
		conn.InCh <- raw
	}
	s.mu.Unlock()
	return nil
}
