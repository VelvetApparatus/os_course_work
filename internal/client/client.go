package client

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"io"
	"net"
	"os_coursach/internal/server/models"
	"sync"
	"time"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var (
	ConnectFailedErr        = errors.New("failed to connect to server")
	ConnectionAlreadySetErr = errors.New("connection already set")
)

type Sets struct {
	AUrl string
	BUrl string
}

type Client struct {
	sets Sets

	connA net.Conn
	chanA chan []byte
	connB net.Conn
	chanB chan []byte

	mu sync.Mutex
}

func NewClient() Client {
	return Client{
		sets: Sets{
			AUrl: "localhost:1452",
			BUrl: "localhost:1453",
		},
		chanA: make(chan []byte),
		chanB: make(chan []byte),
	}
}

var (
	logOutput *widget.Entry
)

func (c *Client) connect(url string) (net.Conn, error) {
	conn, err := net.Dial("tcp", url)
	if err != nil {
		return nil, fmt.Errorf("net dial: %w", err)
	}
	return conn, nil
}

func (c *Client) readLoop(conn net.Conn, ch chan<- []byte) {
	reader := bufio.NewReader(conn)
	batch := make([]byte, 1024)
	for {
		n, err := reader.Read(batch)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				appendLog("read loop exiting... reason: connection closed")
				return
			}
			if errors.Is(err, io.EOF) {
				<-time.After(time.Millisecond * 200)
				continue
			}
			appendLog(fmt.Sprintf("failed to read from reader: %s", err.Error()))
			continue
		}

		data := make([]byte, n)
		copy(data, batch)

		ch <- data
	}
}

func (c *Client) ConnectA() error {

	if c.connA != nil {
		return ConnectionAlreadySetErr
	}

	conn, err := c.connect(c.sets.AUrl)
	if err != nil {
		return ConnectFailedErr
	}

	c.connA = conn

	go c.readLoop(c.connA, c.chanA)

	return nil
}

func (c *Client) ConnectB() error {
	if c.connB != nil {
		return ConnectionAlreadySetErr
	}

	conn, err := c.connect(c.sets.BUrl)
	if err != nil {
		return ConnectFailedErr
	}

	c.connB = conn

	go c.readLoop(c.connB, c.chanB)

	return nil
}

func (c *Client) ConnectBoth() error {
	err := c.ConnectA()
	if err != nil && !errors.Is(err, ConnectionAlreadySetErr) {
		return fmt.Errorf("connect A: %w", err)
	}

	err = c.ConnectB()
	if err != nil && !errors.Is(err, ConnectionAlreadySetErr) {
		return fmt.Errorf("connect B: %w", err)
	}
	return nil
}

func (c *Client) DisconnectA() {
	if c.connA == nil {
		return
	}

	err := c.connA.Close()
	if err != nil {
		appendLog(fmt.Sprintf("close connection A: %s", err.Error()))
	}

	c.connA = nil
}

func (c *Client) DisconnectB() {
	if c.connB == nil {
		return
	}

	err := c.connB.Close()
	if err != nil {
		appendLog(fmt.Sprintf("close connection B: %s", err.Error()))
	}

	c.connB = nil
}

func (c *Client) handleMessages() {
	for {
		select {
		case batch := <-c.chanB:
			msg := models.BroadcastMessage{}
			err := json.Unmarshal(batch, &msg)
			if err != nil {
				appendLog(fmt.Sprintf("unmarshal message: %s", err.Error()))
				continue
			}

			appendLog(fmt.Sprintf("[%s] server B: \n%s", msg.Timestamp.String(), msg.Message))
		case batch := <-c.chanA:
			msg := models.BroadcastMessage{}
			err := json.Unmarshal(batch, &msg)
			if err != nil {
				appendLog(fmt.Sprintf("unmarshal message: %s", err.Error()))
				continue
			}

			appendLog(fmt.Sprintf("[%s] server A: \n%s", msg.Timestamp.String(), msg.Message))
		}
	}
}

func (c *Client) DisconnectBoth() {
	c.DisconnectA()
	c.DisconnectB()
}

func RunClient() {

	client := NewClient()
	go client.handleMessages()

	a := app.New()
	w := a.NewWindow("TCP Клиент")

	logOutput = widget.NewMultiLineEntry()
	logOutput.SetMinRowsVisible(15)
	logOutput.TextStyle = fyne.TextStyle{
		Bold:      false,
		Italic:    false,
		Monospace: false,
		Symbol:    false,
		TabWidth:  0,
		Underline: false,
	}

	// Кнопки управления
	btnConnectA := widget.NewButton("Подключиться к A", func() {
		err := client.ConnectA()
		if err != nil {
			if errors.Is(err, ConnectionAlreadySetErr) {
				widget.ShowPopUp(widget.NewLabel("Уже подключено к A"), w.Canvas())
			} else if errors.Is(err, ConnectFailedErr) {
				widget.ShowPopUp(widget.NewLabel("Не удалось подключиться к A"), w.Canvas())
			} else {
				widget.ShowPopUp(widget.NewLabel(fmt.Sprintf("Ошибка подключения к A: %s", err.Error())), w.Canvas())
			}
			appendLog(fmt.Sprintf("connect a: %s", err.Error()))
		}
	})

	btnConnectB := widget.NewButton("Подключиться к B", func() {
		err := client.ConnectB()
		if err != nil {
			if errors.Is(err, ConnectionAlreadySetErr) {
				widget.ShowPopUp(widget.NewLabel("Уже подключено к B"), w.Canvas())
			} else if errors.Is(err, ConnectFailedErr) {
				widget.ShowPopUp(widget.NewLabel("Не удалось подключиться к B"), w.Canvas())
			} else {
				widget.ShowPopUp(widget.NewLabel(fmt.Sprintf("Ошибка подключения к B: %s", err.Error())), w.Canvas())
			}
			appendLog(fmt.Sprintf("connect b: %s", err.Error()))
		}
	})

	btnConnectBoth := widget.NewButton("Подключиться к обоим", func() {
		err := client.ConnectBoth()
		if err != nil {
			if errors.Is(err, ConnectionAlreadySetErr) {
				widget.ShowPopUp(widget.NewLabel("Уже подключено к серверу"), w.Canvas())
			} else if errors.Is(err, ConnectFailedErr) {
				widget.ShowPopUp(widget.NewLabel("Не удалось подключиться к серверам"), w.Canvas())
			} else {
				widget.ShowPopUp(widget.NewLabel(fmt.Sprintf("Ошибка подключения к серверам: %s", err.Error())), w.Canvas())
			}
			appendLog(fmt.Sprintf("connect both: %s", err.Error()))
		}
	})

	btnDisconnectA := widget.NewButton("Отключиться от A", func() {
		client.DisconnectA()
		widget.ShowPopUp(widget.NewLabel("Отключен"), w.Canvas())
	})
	btnDisconnectB := widget.NewButton("Отключиться от B", func() {
		client.DisconnectB()
		widget.ShowPopUp(widget.NewLabel("Отключен"), w.Canvas())
	})

	btnDisconnectBoth := widget.NewButton("Отключиться от обоих", func() {
		client.DisconnectBoth()
		widget.ShowPopUp(widget.NewLabel("Отключен"), w.Canvas())
	})

	controlRow := container.NewGridWithColumns(3,
		btnConnectA, btnConnectB, btnConnectBoth,
	)

	disconnectRow := container.NewGridWithColumns(3,
		btnDisconnectA, btnDisconnectB, btnDisconnectBoth,
	)

	mainLayout := container.NewVBox(
		controlRow,
		disconnectRow,
		logOutput,
	)

	w.SetContent(mainLayout)
	w.Resize(fyne.NewSize(600, 400))
	w.ShowAndRun()
}

func appendLog(text string) {
	logOutput.SetText(logOutput.Text + text + "\n")
}
