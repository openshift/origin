package dbus

import (
	"context"
	"testing"
	"time"
)

type objectGoContextServer struct {
	t     *testing.T
	sleep time.Duration
}

func (o objectGoContextServer) Sleep() *Error {
	o.t.Log("Got object call and sleeping for ", o.sleep)
	time.Sleep(o.sleep)
	o.t.Log("Completed sleeping for ", o.sleep)
	return nil
}

func TestObjectGoWithContextTimeout(t *testing.T) {
	bus, err := SessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}

	name := bus.Names()[0]
	bus.Export(objectGoContextServer{t, time.Second}, "/org/dannin/DBus/Test", "org.dannin.DBus.Test")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case call := <-bus.Object(name, "/org/dannin/DBus/Test").GoWithContext(ctx, "org.dannin.DBus.Test.Sleep", 0, nil).Done:
		if call.Err != ctx.Err() {
			t.Fatal("Expected ", ctx.Err(), " but got ", call.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Expected call to not respond in time")
	}
}

func TestObjectGoWithContext(t *testing.T) {
	bus, err := SessionBus()
	if err != nil {
		t.Fatalf("Unexpected error connecting to session bus: %s", err)
	}

	name := bus.Names()[0]
	bus.Export(objectGoContextServer{t, time.Millisecond}, "/org/dannin/DBus/Test", "org.dannin.DBus.Test")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case call := <-bus.Object(name, "/org/dannin/DBus/Test").GoWithContext(ctx, "org.dannin.DBus.Test.Sleep", 0, nil).Done:
		if call.Err != ctx.Err() {
			t.Fatal("Expected ", ctx.Err(), " but got ", call.Err)
		}
	case <-time.After(time.Second):
		t.Fatal("Expected call to respond in 1 Millisecond")
	}
}
