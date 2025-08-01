package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/valyala/fastjson"
)

type Location struct {
	Region string
	IP     net.IP
	cloud  string
	Error  error
}

type locator struct {
	C  *cache.Cache
	ch chan Location
}

func newLocator() *locator {
	// Create a cache with a default expiration time of 5 minutes, and which
	// purges expired items every 10 minutes
	c := cache.New(5*time.Minute, 10*time.Minute)
	return &locator{ch: nil, C: c}
}

func (l *locator) locate(ipstr string) *Location {
	if x, found := l.C.Get(ipstr); found {
		return x.(*Location)
	}
	l.ch = make(chan Location)
	ip := net.ParseIP(ipstr)
	if ip == nil {
		return &Location{Error: errors.New("couldn't parse ip")}
	}
	go l.checkGCP(ip)
	go l.checkAWS(ip)
	go l.checkDO(ip)
	go l.checkMA(ip)
	for afterCh := time.After(5 * time.Second); ; {
		select {
		case result := <-l.ch:
			fmt.Println("Got:", result)
			l.C.Set(ipstr, &result, cache.DefaultExpiration)
			return &result
		case <-afterCh:
			return &Location{Error: errors.New("couldn't find cloud")}
		}
	}
}

func (l *locator) checkGCP(rawip net.IP) (bool, error) {
	var p fastjson.Parser
	const url = "https://www.gstatic.com/ipranges/cloud.json"
	res, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return false, err
	}
	json, err := p.ParseBytes(data)
	if err != nil {
		return false, err
	}
	prefixes := json.GetArray("prefixes")
	for _, element := range prefixes {
		ip := element.GetStringBytes("ipv4Prefix")
		if ip == nil {
			// Might be ipv6 only instance/service
			continue
		}
		_, ipnetA, _ := net.ParseCIDR(string(ip))
		if ipnetA.Contains(rawip) {
			region := string(element.GetStringBytes("scope"))
			loc := Location{Region: region, IP: rawip, cloud: "GCP", Error: nil}
			if l.ch != nil {
				l.ch <- loc
				close(l.ch)
			}
			return true, nil
		}
	}
	return false, nil
}

func (l *locator) checkAWS(rawip net.IP) (bool, error) {
	var p fastjson.Parser
	const url = "https://ip-ranges.amazonaws.com/ip-ranges.json"
	res, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return false, err
	}
	json, err := p.ParseBytes(data)
	if err != nil {
		return false, err
	}
	prefixes := json.GetArray("prefixes")
	for _, element := range prefixes {
		ip := element.GetStringBytes("ip_prefix")
		_, ipnetA, _ := net.ParseCIDR(string(ip))
		if ipnetA.Contains(rawip) {
			region := string(element.GetStringBytes("region"))
			loc := Location{Region: region, IP: rawip, cloud: "AWS", Error: nil}
			if l.ch != nil {
				l.ch <- loc
				close(l.ch)
			}
			return true, nil
		}
	}

	return false, nil
}

func (l *locator) checkDO(rawip net.IP) (bool, error) {
	const url = "https://digitalocean.com/geo/google.csv"
	res, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	reader := csv.NewReader(res.Body)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
		_, ipnetA, _ := net.ParseCIDR(record[0])
		if ipnetA.Contains(rawip) {
			region := record[2]
			loc := Location{Region: region, IP: rawip, cloud: "DO", Error: nil}
			if l.ch != nil {
				l.ch <- loc
				close(l.ch)
			}
			return true, nil
		}
	}
	return false, nil
}

func (l *locator) checkMA(rawip net.IP) (bool, error) {
	var p fastjson.Parser
	const filePath = "azure_ips.json"
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return false, err
	}
	json, err := p.ParseBytes(data)
	if err != nil {
		return false, err
	}
	values := json.GetArray("values")
	for _, value := range values {
		props := value.Get("properties")
		region := string(props.GetStringBytes("region"))
		prefixes := props.GetArray("addressPrefixes")
		for _, prefix := range prefixes {
			cidr := string(prefix.GetStringBytes())
			_, ipnet, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			if ipnet.Contains(rawip) {
				loc := Location{Region: region, IP: rawip, cloud: "Azure", Error: nil}
				if l.ch != nil {
					l.ch <- loc
					close(l.ch)
				}
				return true, nil
			}
		}
	}
	return false, nil
}
