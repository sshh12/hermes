package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
)

// https://github.com/pion/webrtc/blob/master/examples/pion-to-pion/offer/main.go

func b() {

	sendC = make(map[string][]string)

	var candidatesMux sync.Mutex
	pendingCandidates := make([]*webrtc.ICECandidate, 0)

	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// When an ICE candidate is available send to the other Pion instance
	// the other Pion instance will add this candidate by calling AddICECandidate
	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		candidatesMux.Lock()
		defer candidatesMux.Unlock()

		desc := peerConnection.RemoteDescription()
		if desc == nil {
			pendingCandidates = append(pendingCandidates, c)
		} else if onICECandidateErr := signalCandidate("b1", c); err != nil {
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

		candidatesMux.Lock()
		defer candidatesMux.Unlock()

		for _, c := range pendingCandidates {
			if onICECandidateErr := signalCandidate("b2", c); onICECandidateErr != nil {
				panic(onICECandidateErr)
			}
		}
	}

	// Create a datachannel with label 'data'
	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		panic(err)
	}

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	dataChan := make(chan []byte, 10)

	// Register channel opening handling
	dataChannel.OnOpen(func() {
		fmt.Printf("Data channel '%s'-'%d' open. Random messages will now be sent to any connected DataChannels every 5 seconds\n", dataChannel.Label(), dataChannel.ID())

		l, err := net.Listen("tcp", ":8000")
		if err != nil {
			fmt.Println(err)
			return
		}
		defer l.Close()

		tmp := make([]byte, 512)
		conn, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}
		go func() {
			for gram := range dataChan {
				fmt.Print(string(gram))
				conn.Write(gram)
			}
		}()
		for {
			n, err := conn.Read(tmp)
			if err != nil {
				break
			}
			dataChannel.Send(tmp[:n])
		}

	})

	// Register text message handling
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		// fmt.Printf("Message from DataChannel '%s': '%s'\n", dataChannel.Label(), string(msg.Data))
		fmt.Println("****", string(msg.Data))
		dataChan <- msg.Data
	})

	// Create an offer to send to the other process
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	// Sets the LocalDescription, and starts our UDP listeners
	// Note: this will start the gathering of ICE candidates
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	// Send our offer to the HTTP server listening in the other process
	payload, err := json.Marshal(offer)
	if err != nil {
		panic(err)
	}
	ioutil.WriteFile("b.sdp", payload, 0o644)

	aSDP := true
	aCan := true

	for {
		if aSDP && fileExists("a.sdp") {
			data, _ := ioutil.ReadFile("a.sdp")
			handleSDP(data)
			aSDP = false
		}
		if aCan && fileExists("a1.sc") {
			data, _ := ioutil.ReadFile("a1.sc")
			for _, line := range strings.Split(string(data), "\n") {
				handleCandidate(line)
			}
			aCan = false
		}
		time.Sleep(1 * time.Second)
	}
}
