package main

import (
	"fmt"
	"net"
	"testing"
)

func TestCheckGCP(t *testing.T) {
	ip := net.ParseIP("8.34.212.6")
	if ip == nil {
		t.Errorf("Couldn't parse ip")
		return
	}
	l := locator{}
	res, err := l.checkGCP(ip)
	if err != nil {
		t.Errorf("Got error when not expected")
		return
	}
	if !res {
		fmt.Println(res)
		t.Errorf("GCP didn't find ip!; should've been found")
		return
	}

}
func TestCheckAWS(t *testing.T) {
	ip := net.ParseIP("54.222.96.0")
	if ip == nil {
		t.Errorf("Couldn't parse ip")
		return
	}
	l := locator{}
	res, err := l.checkAWS(ip)
	if err != nil {
		t.Errorf("Got error when not expected")
		return
	}
	if !res {
		t.Errorf("AWS didn't find ip!; should've been found")
		return
	}
}
func TestCheckDO(t *testing.T) {
	ip := net.ParseIP("24.199.64.7")
	if ip == nil {
		t.Errorf("Couldn't parse ip")
		return
	}
	l := locator{}
	res, err := l.checkDO(ip)
	if err != nil {
		t.Errorf("Got error when not expected")
		return
	}
	if !res {
		t.Errorf("DO didn't find ip!; should've been found")
		return
	}
}
func TestCheckMA(t *testing.T) {
	ip := net.ParseIP("4.221.116.6")
	if ip == nil {
		t.Errorf("Couldn't parse ip")
		return
	}
	l := locator{}
	res, err := l.checkMA(ip)
	if err != nil {
		t.Errorf("Got error when not expected")
		return
	}
	if !res {
		t.Errorf("MA didn't find ip!; should've been found")
		return
	}
}
