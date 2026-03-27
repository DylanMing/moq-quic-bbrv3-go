package api

import (
	"context"

	"github.com/DineshAdhi/moq-go/moqt"
	"github.com/DineshAdhi/moq-go/moqt/wire"
)

type MOQSub struct {
	Options           moqt.DialerOptions
	Relay             string
	Ctx               context.Context
	onStreamHandler   func(moqt.SubStream)
	onAnnounceHandler func(string)
	handler           *moqt.SubHandler
	session           *moqt.MOQTSession
}

func NewMOQSub(options moqt.DialerOptions, relay string) *MOQSub {
	sub := &MOQSub{
		Options: options,
		Relay:   relay,
		Ctx:     context.TODO(),
	}

	return sub
}

func (sub *MOQSub) OnStream(f func(moqt.SubStream)) {
	sub.onStreamHandler = f
}

func (sub *MOQSub) OnAnnounce(f func(string)) {
	sub.onAnnounceHandler = f
}

func (sub *MOQSub) Connect() (*moqt.SubHandler, error) {

	dialer := moqt.MOQTDialer{
		Options: sub.Options,
		Role:    wire.ROLE_SUBSCRIBER,
		Ctx:     sub.Ctx,
	}

	session, err := dialer.Dial(sub.Relay)

	if err != nil {
		return nil, err
	}

	sub.session = session
	handler := session.SubHandler()

	go func() {
		for stream := range handler.StreamsChan {
			sub.onStreamHandler(stream)
		}
	}()

	go func() {
		for ns := range handler.AnnounceChan {
			sub.onAnnounceHandler(ns)
		}
	}()

	return handler, nil
}

func (sub *MOQSub) GetConnectionStats() moqt.ConnectionStats {
	if sub.session != nil {
		return sub.session.GetConnectionStats()
	}
	return moqt.ConnectionStats{}
}
