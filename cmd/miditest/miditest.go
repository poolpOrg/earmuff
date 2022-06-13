package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/poolpOrg/earring/midi"
	"github.com/poolpOrg/earring/parser"
	"github.com/youpy/go-coremidi"
	"gitlab.com/gomidi/midi/v2/smf"
	// (Meta Messages)
	// you may also want to use these
	// github.com/gomidi/midi/midimessage/cc         (ControlChange Messages)
	// github.com/gomidi/midi/midimessage/sysex      (System Exclusive Messages)
)

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		log.Fatal("need a source file to process")
	}

	fp, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer fp.Close()

	parser := parser.NewParser(fp)
	project, err := parser.Parse()
	if err != nil {
		log.Fatal(err)
	}

	b := midi.ToMidi(project)

	//fp2, err := os.OpenFile("file.mid", os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//defer fp2.Close()
	///
	//fp2.Write(b)
	client, err := coremidi.NewClient("earring")
	if err != nil {
		fmt.Println(err)
		return
	}

	port, err := coremidi.NewInputPort(
		client,
		"test",
		func(source coremidi.Source, packet coremidi.Packet) {
			fmt.Printf(
				"device: %v, manufacturer: %v, source: %v, data: %v\n",
				source.Entity().Device().Name(),
				source.Manufacturer(),
				source.Name(),
				packet.Data,
			)
			return
		},
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	source, err := coremidi.NewSource(client, "earring")
	if err != nil {
		fmt.Println(err)
		return
	}
	conn, err := port.Connect(source)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(conn)

	e, err := smf.ReadFrom(bytes.NewReader(b))
	if err != nil {
		fmt.Println(err)
		return
	}

	tracks := e.Tracks

	//out, _ := coremidi.NewOutputPort(client, "out")

	for _, track := range tracks {
		for _, ev := range track {
			//out.WriteString(fmt.Sprintf("Track %v@%v %s\n", i, ev.Delta, ev.MessageType()))
			//m := midi2.NewMessage(ev.Data)
			//m.Type = midi2.GetMessageType(ev.Data)
			p := coremidi.NewPacket(ev.Message.Bytes(), uint64(ev.Delta))
			//			p.Send(conn, )
			fmt.Println(p)

		}
	}

	conn.Disconnect()
	_ = b

	ch := make(chan int)
	<-ch

}
