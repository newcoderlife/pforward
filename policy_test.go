package pforward

import (
	"fmt"
	"testing"
)

func TestPolicy_SelectServer(t *testing.T) {
	p := MakePolicy()
	p.AddRule(".", "114.114.114.114")
	p.AddRule("google.com.", "1.1.1.1")

	fmt.Println(p.SelectServer("www.google.com."))
	fmt.Println(p.SelectServer("www.baidu.com."))
}
