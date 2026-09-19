package sms

import "testing"

func TestFromEnvLogSenderWithoutGateway(t *testing.T) {
	sender, debug, err := FromEnv(func(string) string { return "" })
	if err != nil || debug != nil {
		t.Fatalf("%v %v", debug, err)
	}
	if _, ok := sender.(LogSender); !ok {
		t.Fatalf("%T", sender)
	}
}

func TestFromEnvRefusesADebugCodeBesideAGateway(t *testing.T) {
	getenv := func(name string) string {
		switch name {
		case GatewayURLEnv:
			return "https://sms.example.test/send"
		case GatewayTokenEnv:
			return "tok"
		case DebugCodeEnv:
			return "000" + "000"
		}
		return ""
	}
	_, _, err := FromEnv(getenv)
	if err == nil {
		t.Fatal("expected ConfigInvalid")
	}
	if _, ok := err.(*ConfigInvalid); !ok {
		t.Fatalf("%T %v", err, err)
	}
}

func TestFromEnvHonoursASixDigitDebugCodeBesideTheLogSender(t *testing.T) {
	code := "000" + "000"
	getenv := func(name string) string {
		if name == DebugCodeEnv {
			return code
		}
		return ""
	}
	sender, debug, err := FromEnv(getenv)
	if err != nil || debug == nil || *debug != code {
		t.Fatalf("%v %v %v", sender, debug, err)
	}
	if _, ok := sender.(LogSender); !ok {
		t.Fatalf("%T", sender)
	}
}
