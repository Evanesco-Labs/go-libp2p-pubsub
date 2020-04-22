package pubsub

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
)

func TestScoreTimeInMesh(t *testing.T) {
	// Create parameters with reasonable default values
	mytopic := "mytopic"
	params := &PeerScoreParams{
		AppSpecificScore: func(peer.ID) float64 { return 0 },
		Topics:           make(map[string]*TopicScoreParams),
	}
	topicScoreParams := &TopicScoreParams{
		TopicWeight:       0.5,
		TimeInMeshWeight:  1,
		TimeInMeshQuantum: time.Millisecond,
		TimeInMeshCap:     3600,
	}
	params.Topics[mytopic] = topicScoreParams

	peerA := peer.ID("A")

	// Peer score should start at 0
	ps := newPeerScore(params)
	ps.AddPeer(peerA, "myproto")

	aScore := ps.Score(peerA)
	if aScore != 0 {
		t.Fatal("expected score to start at zero")
	}

	// The time in mesh depends on how long the peer has been grafted
	ps.Graft(peerA, mytopic)
	elapsed := topicScoreParams.TimeInMeshQuantum * 40
	time.Sleep(elapsed)

	ps.refreshScores()
	aScore = ps.Score(peerA)
	expected := topicScoreParams.TopicWeight * topicScoreParams.TimeInMeshWeight * float64(elapsed/topicScoreParams.TimeInMeshQuantum)
	variance := 0.5
	if !withinVariance(aScore, expected, variance) {
		t.Fatalf("Score: %f. Expected %f ± %f", aScore, expected, variance*expected)
	}
}

func TestScoreTimeInMeshCap(t *testing.T) {
	// Create parameters with reasonable default values
	mytopic := "mytopic"
	params := &PeerScoreParams{
		AppSpecificScore: func(peer.ID) float64 { return 0 },
		Topics:           make(map[string]*TopicScoreParams),
	}
	topicScoreParams := &TopicScoreParams{
		TopicWeight:       0.5,
		TimeInMeshWeight:  1,
		TimeInMeshQuantum: time.Millisecond,
		TimeInMeshCap:     10,
	}

	params.Topics[mytopic] = topicScoreParams

	peerA := peer.ID("A")

	ps := newPeerScore(params)
	ps.AddPeer(peerA, "myproto")
	ps.Graft(peerA, mytopic)
	elapsed := topicScoreParams.TimeInMeshQuantum * 40
	time.Sleep(elapsed)

	// The time in mesh score has a cap
	ps.refreshScores()
	aScore := ps.Score(peerA)
	expected := topicScoreParams.TopicWeight * topicScoreParams.TimeInMeshWeight * topicScoreParams.TimeInMeshCap
	variance := 0.5
	if !withinVariance(aScore, expected, variance) {
		t.Fatalf("Score: %f. Expected %f ± %f", aScore, expected, variance*expected)
	}
}

func TestScoreFirstMessageDeliveries(t *testing.T) {
	// Create parameters with reasonable default values
	mytopic := "mytopic"
	params := &PeerScoreParams{
		AppSpecificScore: func(peer.ID) float64 { return 0 },
		Topics:           make(map[string]*TopicScoreParams),
	}
	topicScoreParams := &TopicScoreParams{
		TopicWeight:       1,
		FirstMessageDeliveriesWeight:  1,
		FirstMessageDeliveriesDecay: 1.0, // test without decay for now
		FirstMessageDeliveriesCap: 2000,
		TimeInMeshQuantum: time.Second, // bug? not setting this causes a div by zero
	}

	params.Topics[mytopic] = topicScoreParams
	peerA := peer.ID("A")

	ps := newPeerScore(params)
	ps.AddPeer(peerA, "myproto")
	ps.Graft(peerA, mytopic)
	
	// deliver a bunch of messages from peer A
	nMessages := 100
	for i := 0; i < nMessages; i++ {
		pbMsg := makeTestMessage(i)
		pbMsg.TopicIDs = []string{mytopic}
		msg := Message{ReceivedFrom: peerA, Message: pbMsg}
		ps.ValidateMessage(&msg)
		ps.DeliverMessage(&msg)
	}

	ps.refreshScores()
	aScore := ps.Score(peerA)
	expected := topicScoreParams.TopicWeight * topicScoreParams.FirstMessageDeliveriesWeight * float64(nMessages)
	variance := 0.5
	if !withinVariance(aScore, expected, variance) {
		t.Fatalf("Score: %f. Expected %f ± %f", aScore, expected, variance*expected)
	}
}

func TestScoreFirstMessageDeliveriesCap(t *testing.T) {
	// Create parameters with reasonable default values
	mytopic := "mytopic"
	params := &PeerScoreParams{
		AppSpecificScore: func(peer.ID) float64 { return 0 },
		Topics:           make(map[string]*TopicScoreParams),
	}
	topicScoreParams := &TopicScoreParams{
		TopicWeight:       1,
		FirstMessageDeliveriesWeight:  1,
		FirstMessageDeliveriesDecay: 1.0, // test without decay for now
		FirstMessageDeliveriesCap: 50,
		TimeInMeshQuantum: time.Second, // bug? not setting this causes a div by zero
	}

	params.Topics[mytopic] = topicScoreParams
	peerA := peer.ID("A")

	ps := newPeerScore(params)
	ps.AddPeer(peerA, "myproto")
	ps.Graft(peerA, mytopic)

	// deliver a bunch of messages from peer A
	nMessages := 100
	for i := 0; i < nMessages; i++ {
		pbMsg := makeTestMessage(i)
		pbMsg.TopicIDs = []string{mytopic}
		msg := Message{ReceivedFrom: peerA, Message: pbMsg}
		ps.ValidateMessage(&msg)
		ps.DeliverMessage(&msg)
	}

	ps.refreshScores()
	aScore := ps.Score(peerA)
	expected := topicScoreParams.TopicWeight * topicScoreParams.FirstMessageDeliveriesWeight * topicScoreParams.FirstMessageDeliveriesCap
	variance := 0.5
	if !withinVariance(aScore, expected, variance) {
		t.Fatalf("Score: %f. Expected %f ± %f", aScore, expected, variance*expected)
	}
}

func TestScoreFirstMessageDeliveriesDecay(t *testing.T) {
	// Create parameters with reasonable default values
	mytopic := "mytopic"
	params := &PeerScoreParams{
		AppSpecificScore: func(peer.ID) float64 { return 0 },
		Topics:           make(map[string]*TopicScoreParams),
	}
	topicScoreParams := &TopicScoreParams{
		TopicWeight:                  1,
		FirstMessageDeliveriesWeight: 1,
		FirstMessageDeliveriesDecay:  0.9, // decay 10% per decay interval
		FirstMessageDeliveriesCap:    2000,
		TimeInMeshQuantum:            time.Second, // bug? not setting this causes a div by zero
	}

	params.Topics[mytopic] = topicScoreParams
	peerA := peer.ID("A")

	ps := newPeerScore(params)
	ps.AddPeer(peerA, "myproto")
	ps.Graft(peerA, mytopic)

	// deliver a bunch of messages from peer A
	nMessages := 100
	for i := 0; i < nMessages; i++ {
		pbMsg := makeTestMessage(i)
		pbMsg.TopicIDs = []string{mytopic}
		msg := Message{ReceivedFrom: peerA, Message: pbMsg}
		ps.ValidateMessage(&msg)
		ps.DeliverMessage(&msg)
	}

	ps.refreshScores()
	aScore := ps.Score(peerA)
	expected := topicScoreParams.TopicWeight * topicScoreParams.FirstMessageDeliveriesWeight * topicScoreParams.FirstMessageDeliveriesDecay * float64(nMessages)
	variance := 0.1
	if !withinVariance(aScore, expected, variance) {
		t.Fatalf("Score: %f. Expected %f ± %f", aScore, expected, variance*expected)
	}

	// refreshing the scores applies the decay param, so applying twice should
	decayIntervals := 10
	for i := 0; i < decayIntervals; i++ {
		ps.refreshScores()
		expected *= topicScoreParams.FirstMessageDeliveriesDecay
	}
	aScore = ps.Score(peerA)
	if !withinVariance(aScore, expected, variance) {
		t.Fatalf("Score: %f. Expected %f ± %f", aScore, expected, variance*expected)
	}
}

func withinVariance(score float64, expected float64, variance float64) bool {
	return score > expected*(1-variance) && score < expected*(1+variance)
}
