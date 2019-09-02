package restful

import (
	"testing"
	"time"
)

func TestRouteBuilder_PathParameter(t *testing.T) {
	p := &Parameter{&ParameterData{Name: "name", Description: "desc"}}
	p.AllowMultiple(true)
	p.DataType("int")
	p.Required(true)
	values := map[string]string{"a": "b"}
	p.AllowableValues(values)
	p.bePath()

	b := new(RouteBuilder)
	b.function = dummy
	b.Param(p)
	r := b.Build()
	if !r.ParameterDocs[0].Data().AllowMultiple {
		t.Error("AllowMultiple invalid")
	}
	if r.ParameterDocs[0].Data().DataType != "int" {
		t.Error("dataType invalid")
	}
	if !r.ParameterDocs[0].Data().Required {
		t.Error("required invalid")
	}
	if r.ParameterDocs[0].Data().Kind != PathParameterKind {
		t.Error("kind invalid")
	}
	if r.ParameterDocs[0].Data().AllowableValues["a"] != "b" {
		t.Error("allowableValues invalid")
	}
	if b.ParameterNamed("name") == nil {
		t.Error("access to parameter failed")
	}
}

func TestRouteBuilder(t *testing.T) {
	json := "application/json"
	b := new(RouteBuilder)
	b.To(dummy)
	b.Path("/routes").Method("HEAD").Consumes(json).Produces(json).Metadata("test", "test-value").DefaultReturns("default", time.Now())
	r := b.Build()
	if r.Path != "/routes" {
		t.Error("path invalid")
	}
	if r.Produces[0] != json {
		t.Error("produces invalid")
	}
	if r.Consumes[0] != json {
		t.Error("consumes invalid")
	}
	if r.Operation != "dummy" {
		t.Error("Operation not set")
	}
	if r.Metadata["test"] != "test-value" {
		t.Errorf("Metadata not set")
	}
	if r.DefaultResponse == nil {
		t.Fatal("expected default response")
	}
}

func TestAnonymousFuncNaming(t *testing.T) {
	f1 := func() {}
	f2 := func() {}
	if got, want := nameOfFunction(f1), "func1"; got != want {
		t.Errorf("got %v want %v", got, want)
	}
	if got, want := nameOfFunction(f2), "func2"; got != want {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestContentEncodingEnabled(t *testing.T) {
	b := new(RouteBuilder)
	b.function = dummy
	r := b.Build()

	got := r.contentEncodingEnabled
	var want *bool //nil

	if got != want {
		t.Errorf("got %v want %v (default nil)", got, want)
	}

	//true
	b = new(RouteBuilder)
	b.function = dummy
	b.ContentEncodingEnabled(true)
	r = b.Build()
	got = r.contentEncodingEnabled

	if *got != true {
		t.Errorf("got %v want %v (explicit true)", *got, true)
	}

	//true
	b = new(RouteBuilder)
	b.function = dummy
	b.ContentEncodingEnabled(false)
	r = b.Build()
	got = r.contentEncodingEnabled

	if *got != false {
		t.Errorf("got %v want %v (explicit false)", *got, false)
	}

}
