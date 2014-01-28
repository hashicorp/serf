package command

import (
	"testing"
	"fmt"
)

type OutputTest struct {
	XMLName    string           `json:"-"           xml:"test"`
	TestString string           `json:"test_string" xml:"test_string"`
	TestInt    int              `json:"test_int"    xml:"test_int"`
	TestNil    []byte           `json:"test_nil"    xml:"test_nil"`
	TestNested OutputTestNested `json:"nested"      xml:"nested"`
}

type OutputTestNested struct {
	NestKey string `json:"nest_key" xml:"nest_key"`
}

func (o OutputTest) String() string {
	return fmt.Sprintf("%s    %d    %s", o.TestString, o.TestInt, o.TestNil)
}

func TestCommandOutput(t *testing.T) {
	var formatted []byte
	result := OutputTest{
		TestString: "woooo a string",
		TestInt:    77,
		TestNil:    nil,
		TestNested: OutputTestNested{
			NestKey: "nest_value",
		},
	}

	json_expected := `{
  "test_string": "woooo a string",
  "test_int": 77,
  "test_nil": null,
  "nested": {
    "nest_key": "nest_value"
  }
}`
	formatted, _ = formatOutput(result, "json")
	if string(formatted) != json_expected {
		t.Fatalf("bad json:\n%s\n\nexpected:\n%s", formatted, json_expected)
	}

	xml_expected := `<?xml version="1.0" encoding="UTF-8"?>
<test>
  <test_string>woooo a string</test_string>
  <test_int>77</test_int>
  <test_nil></test_nil>
  <nested>
    <nest_key>nest_value</nest_key>
  </nested>
</test>`
	formatted, _ = formatOutput(result, "xml")
	if string(formatted) != xml_expected {
		t.Fatalf("bad xml:\n%s\n\nexpected:\n%s", formatted, xml_expected)
	}

	text_expected := "woooo a string    77"
	formatted, _ = formatOutput(result, "text")
	if string(formatted) != text_expected {
		t.Fatalf("bad output:\n\"%s\"\n\nexpected:\n\"%s\"", formatted, text_expected)
	}

	error_expected := `Invalid output format "boo"`
	_, err := formatOutput(result, "boo")
	if err.Error() != error_expected {
		t.Fatalf("bad output:\n\"%s\"\n\nexpected:\n\"%s\"", err.Error(), error_expected)
	}
}
