package main

import (
	"io/ioutil"
	"net"
	"net/http"
	"sync"

	"github.com/valyala/fastjson"
)

type locater struct {
	IP net.IP
	WG sync.WaitGroup
}

func locate(ip string) {
	ip_parsed := net.ParseIP(ip)
	l := locater{IP: ip_parsed}
	l.WG.Add(2)
	go l.checkGCP()
	go l.checkAWS()
	l.WG.Wait()
}

func (l *locater) checkGCP() (error, bool) {
	defer l.WG.Done()
	var p fastjson.Parser
	const url = "https://www.gstatic.com/ipranges/cloud.json"
	res, err := http.Get(url)
	defer res.Body.Close()
	if err != nil {
		return err, false
	}
	data, err := ioutil.ReadAll(res.Body)
	json, err := p.ParseBytes(data)
	if err != nil {
		return err, false
	}
	prefixes := json.GetArray("prefixes")
	for _, element := range prefixes {
		ip := element.GetStringBytes("ipv4Prefix")
		_, ipnetA, _ := net.ParseCIDR(string(ip[:]))
		if ipnetA.Contains(l.IP) {
			return nil, true
		}
	}
	return nil, false
}

func (l *locater) checkAWS() (error, bool) {
	defer l.WG.Done()
	var p fastjson.Parser
	const url = "https://ip-ranges.amazonaws.com/ip-ranges.json"
	res, err := http.Get(url)
	defer res.Body.Close()
	if err != nil {
		return err, false
	}
	data, err := ioutil.ReadAll(res.Body)
	json, err := p.ParseBytes(data)
	if err != nil {
		return err, false
	}
	prefixes := json.GetArray("prefixes")
	for _, element := range prefixes {
		ip := element.GetStringBytes("ip_prefix")
		_, ipnetA, _ := net.ParseCIDR(string(ip[:]))
		if ipnetA.Contains(l.IP) {
			return nil, true
		}
	}
	return nil, false
}
