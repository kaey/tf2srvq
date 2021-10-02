package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

func main() {
	if err := Main(); err != nil {
		log.Fatalln(err)
	}
}

type Server struct {
	Addr    string `json:"addr"`
	Comment string `json:"comment"`
}

type GetResult struct {
	info    *Info
	err     error
	players []string
	err2    error
}

func Main() error {
	data, err := os.ReadFile("config.json")
	if err != nil {
		return err
	}

	config := struct {
		Srvs []Server `json:"servers"`
	}{}
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	srvs := config.Srvs

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res := make([]GetResult, len(srvs))
	wg := new(sync.WaitGroup)
	for i := range srvs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			info, err := GetInfo(ctx, srvs[i].Addr)
			pls, err2 := GetPlayersInfo(ctx, srvs[i].Addr)
			res[i] = GetResult{info: info, err: err, players: pls, err2: err2}
		}(i)
	}

	wg.Wait()

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 1, ' ', 0)
	for i := range res {
		addr := srvs[i].Addr
		if err := res[i].err; err != nil {
			fmt.Fprintf(tw, "steam://connect/%s\t%v\n", addr, err)
			continue
		}

		name := res[i].info.Name
		mp := res[i].info.Map
		pl := res[i].info.PlayersCurrent - res[i].info.PlayersBots
		max := res[i].info.PlayersMax

		pls := ""
		if res[i].err2 != nil {
			pls = res[i].err2.Error()
		} else {
			n := len(res[i].players)
			if n > 6 {
				n = 6
			}
			pls = strings.Join(res[i].players[:n], ",")
		}

		fmt.Fprintf(tw, "steam://connect/%s\t%s\t%s\t%d/%d\t%s\n", addr, name, mp, pl, max, pls)
	}

	return tw.Flush()
}

type Info struct {
	Name           string
	Map            string
	Game           string
	PlayersCurrent int
	PlayersMax     int
	PlayersBots    int
	Type           byte
	OS             byte
	Password       bool
	VAC            bool
}

func GetInfo(ctx context.Context, addr string) (*Info, error) {
	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpaddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	dl, ok := ctx.Deadline()
	if ok {
		if err := conn.SetDeadline(dl); err != nil {
			return nil, err
		}
	}
	req := []byte("\xFF\xFF\xFF\xFFTSource Engine Query\x00")
	var chlg []byte
	resp := make([]byte, 1500)
	var resperr error
	for i := 0; i < 5; i++ {
		req := append(req, chlg...)
		if _, err := conn.Write(req); err != nil {
			resperr = err
			continue
		}

		r := resp
		if _, err := conn.Read(resp); err != nil {
			resperr = err
			continue
		}

		if !bytes.Equal(getBytes(&r, 4), []byte("\xFF\xFF\xFF\xFF")) {
			resperr = fmt.Errorf("split packet not supported")
			continue
		}

		typ := getByte(&r)
		if typ == 'A' {
			resperr = fmt.Errorf("expect info message, got %c", typ)
			chlg = getBytes(&r, 4)
			continue
		}

		if typ != 'I' {
			resperr = fmt.Errorf("expect info message, got %c", typ)
			continue
		}

		resp, resperr = r, nil
		break
	}

	if resperr != nil {
		return nil, resperr
	}

	getByte(&resp) // Protocol
	name := getString(&resp)
	mp := getString(&resp)
	getString(&resp) // Folder
	game := getString(&resp)
	getShort(&resp) // AppID
	players := int(getByte(&resp))
	maxplayers := int(getByte(&resp))
	bots := int(getByte(&resp))
	typ := getByte(&resp)
	os := getByte(&resp)
	password := getBool(&resp)
	vac := getBool(&resp)

	return &Info{
		Name:           name,
		Map:            mp,
		Game:           game,
		PlayersCurrent: players,
		PlayersMax:     maxplayers,
		PlayersBots:    bots,
		Type:           typ,
		OS:             os,
		Password:       password,
		VAC:            vac,
	}, nil
}

func GetPlayersInfo(ctx context.Context, addr string) ([]string, error) {
	udpaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpaddr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	dl, ok := ctx.Deadline()
	if ok {
		if err := conn.SetDeadline(dl); err != nil {
			return nil, err
		}
	}
	req := []byte("\xFF\xFF\xFF\xFFU")
	chlg := []byte("\xFF\xFF\xFF\xFF")
	resp := make([]byte, 1500)
	var resperr error
	for i := 0; i < 5; i++ {
		req := append(req, chlg...)
		if _, err := conn.Write(req); err != nil {
			resperr = err
			continue
		}

		r := resp
		if _, err := conn.Read(resp); err != nil {
			resperr = err
			continue
		}

		if !bytes.Equal(getBytes(&r, 4), []byte("\xFF\xFF\xFF\xFF")) {
			resperr = fmt.Errorf("split packet not supported")
			continue
		}

		typ := getByte(&r)
		if typ == 'A' {
			resperr = fmt.Errorf("expect players info message, got %c", typ)
			chlg = getBytes(&r, 4)
			continue
		}

		if typ != 'D' {
			resperr = fmt.Errorf("expect players info message, got %c", typ)
			continue
		}

		resp, resperr = r, nil
		break
	}

	if resperr != nil {
		return nil, resperr
	}

	npls := int(getByte(&resp))
	pls := make([]string, npls)
	for i := 0; i < npls; i++ {
		getByte(&resp) // Index
		name := getString(&resp)
		getLong(&resp)  // Score
		getFloat(&resp) // Duration
		pls[i] = name
	}

	return pls, nil
}
