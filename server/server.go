package server

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	pb "github.com/ldelossa/gGrok/proto"
)

type Server struct {
	// Server will be listening for incoming HTTP requests.
	*http.Server
	// Locks streamMap
	sync.RWMutex
	// Holds a mapping between hostnames and associated streams
	streamMap map[string]pb.GGrok_StreamServer
}

func (s *Server) Stream(stream pb.GGrok_StreamServer) error {
	// Receive message
	m, err := stream.Recv()
	if err != nil {
		e := fmt.Sprintf("received error on receive: %s", err)
		log.Printf(e)
		return fmt.Errorf(e)
	}

	// Expect message to InitRequest
	switch mm := m.Clientmsg.(type) {
	case *pb.ClientServer_Initreq:
		// if we receive a hostname in the initial request message attempt to register stream with this hostname
		if mm.Initreq.Hostname != "" {
			// determine if hostname is taken
			s.RLock()
			_, ok := s.streamMap[mm.Initreq.Hostname]
			s.Unlock()

			// if hostname not taken
			if !ok {
				// take lock and check again to make sure no other routine has added the hostname yet
				s.Lock()
				_, ok := s.streamMap[mm.Initreq.Hostname]
				if !ok {
					// register stream
					s.streamMap[mm.Initreq.Hostname] = stream
					s.Unlock()
					log.Printf("registered stream with requested hostname %s", mm.Initreq.Hostname)
					return nil
				}
				s.Unlock()
			}

		}

		// No hostname in request or failed to register requested hostname
		uu := uuid.New().String()
		s.Lock()
		s.streamMap[uu] = stream
		s.Unlock()
		log.Printf("registered stream with generated hostname %s", uu)
		return nil
	default:
		e := fmt.Sprintf("expected message type ClientServer_Initreq but got %T", mm)
		log.Printf(e)
		return fmt.Errorf(e)
	}
}
