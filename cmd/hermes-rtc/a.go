package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
)

// https://github.com/pion/webrtc/blob/master/examples/pion-to-pion/answer/main.go

var sendC map[string][]string

func signalCandidate(code string, c *webrtc.ICECandidate) error {
	if sendC[code] == nil {
		sendC[code] = make([]string, 0)
	}
	sendC[code] = append(sendC[code], c.ToJSON().Candidate)
	ioutil.WriteFile(code+".sc", []byte(strings.Join(sendC[code], "\n")), 0o644)
	fmt.Println(sendC)
	return nil
}

func a() {

	sendC = make(map[string][]string)

	var candidatesMux sync.Mutex
	pendingCandidates := make([]*webrtc.ICECandidate, 0)

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		candidatesMux.Lock()
		defer candidatesMux.Unlock()

		desc := peerConnection.RemoteDescription()
		if desc == nil {
			pendingCandidates = append(pendingCandidates, c)
		} else if onICECandidateErr := signalCandidate("a1", c); onICECandidateErr != nil {
			panic(onICECandidateErr)
		}
	})

	handleCandidate := func(candVal string) {
		if candidateErr := peerConnection.AddICECandidate(webrtc.ICECandidateInit{Candidate: candVal}); candidateErr != nil {
			panic(candidateErr)
		}
	}

	handleSDP := func(sdpVal []byte) {
		sdp := webrtc.SessionDescription{}
		if err := json.Unmarshal(sdpVal, &sdp); err != nil {
			panic(err)
		}

		if err := peerConnection.SetRemoteDescription(sdp); err != nil {
			panic(err)
		}

		// Create an answer to send to the other process
		answer, err := peerConnection.CreateAnswer(nil)
		if err != nil {
			panic(err)
		}

		// Send our answer to the HTTP server listening in the other process
		payload, err := json.Marshal(answer)
		ioutil.WriteFile("a.sdp", payload, 0o644)

		// Sets the LocalDescription, and starts our UDP listeners
		err = peerConnection.SetLocalDescription(answer)
		if err != nil {
			panic(err)
		}

		candidatesMux.Lock()
		for _, c := range pendingCandidates {
			onICECandidateErr := signalCandidate("a2", c)
			if onICECandidateErr != nil {
				panic(onICECandidateErr)
			}
		}
		candidatesMux.Unlock()
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// Register data channel creation handling
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		fmt.Printf("New DataChannel %s %d\n", d.Label(), d.ID())

		conn, err := net.Dial("tcp", "localhost:8001")
		if err != nil {
			panic(err)
		}

		dataChan := make(chan []byte, 10)

		go func() {
			tmp := make([]byte, 512)
			for {
				n, err := conn.Read(tmp)
				if err != nil {
					break
				}
				dataChan <- tmp[:n]
			}
		}()

		// Register channel opening handling
		d.OnOpen(func() {
			fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", d.Label(), d.ID())

			for gram := range dataChan {
				d.Send(gram)
			}
		})

		// Register text message handling
		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			fmt.Print(string(msg.Data))
			conn.Write(msg.Data)
		})
	})

	bSDP := true
	bCan := true

	for {
		if bSDP && fileExists("b.sdp") {
			data, _ := ioutil.ReadFile("b.sdp")
			handleSDP(data)
			bSDP = false
		}
		if bCan && fileExists("b2.sc") {
			data, _ := ioutil.ReadFile("b2.sc")
			for _, line := range strings.Split(string(data), "\n") {
				handleCandidate(line)
			}
			bCan = false
		}
		time.Sleep(1 * time.Second)
	}

	// reader := bufio.NewReader(os.Stdin)
	// fmt.Print("Enter SDP: ")
	// text, _ := reader.ReadString('\n')
	// fmt.Println(text)
	// handleSDP(text)
	// fmt.Print("Enter Canidate: ")
	// text2, _ := reader.ReadString('\n')
	// fmt.Println(text2)
	// handleCandidate(text2)

}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
