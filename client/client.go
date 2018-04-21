package client

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"

	pb "github.com/ldelossa/gGrok/proto"
)

type Proxyer interface {
	Proxy() error
}

type Client struct {
	stream      pb.GGrok_StreamClient
	httpClient  *http.Client
	proxyTarget *url.URL
}

func NewClient(stream pb.GGrok_StreamClient, httpClient *http.Client, proxyTarget *url.URL) *Client {
	return &Client{
		stream:      stream,
		httpClient:  httpClient,
		proxyTarget: proxyTarget,
	}
}

// Proxy implements the Proxyer interface. Method begins
func (c *Client) Proxy() {
	// begin proxy loop
	for {
		msg, err := c.stream.Recv()
		if err != nil {
			log.Printf("failed to call Recv on stream: %s", err)
			return
		}

		// type switch on received msg
		switch x := msg.Servermsg.(type) {
		case *pb.ServerClient_Httpreq:
			// unpack request byte array from proto
			reqBuf := bytes.NewBuffer(x.Httpreq.Request)
			// perform proxy. keeping this synchronous for now because stream.Send() called in this method is not thread safe
			err := c.doProxy(reqBuf)
			if err != nil {
				log.Printf("proxy request failed: %s", err)
				continue
			}
		default:
			log.Printf("received unknown message over stream")
			continue
		}

	}
}

func (c *Client) doProxy(reqBuf io.ReadWriter) error {
	// read reqBuf into http.Request object
	bufioReqBuf := bufio.NewReader(reqBuf)
	req, err := http.ReadRequest(bufioReqBuf)
	if err != nil {
		return err
	}

	// Replace original request details with proxy target's
	req.URL.Host = c.proxyTarget.Host
	req.URL.Scheme = c.proxyTarget.Scheme

	// Make request to target
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	// Wrap resp into ClientServer object and send over stream
	respBuf := &bytes.Buffer{}

	err = resp.Write(respBuf)
	if err != nil {
		return err
	}

	csm := &pb.ClientServer{
		Clientmsg: &pb.ClientServer_Httpresp{
			Httpresp: &pb.HTTPResponse{
				Response: respBuf.Bytes(),
			},
		},
	}

	err = c.stream.Send(csm)
	if err != nil {
		return err
	}

	return nil
}
