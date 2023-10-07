/*
*
current_values_uuid = UUID('b42e2a68-ade7-11e4-89d3-123b93f75cba')
    command_uuid        = UUID('b42e2d06-ade7-11e4-89d3-123b93f75cba')    # "Access Control Point" Characteristic
    sensor_record_uuid  = UUID('b42e2fc2-ade7-11e4-89d3-123b93f75cba')
https://sifter.org/~simon/journal/ims/20191210/AirthingsWavePlusBtle.py

*/
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/paypal/gatt"
	log "github.com/sirupsen/logrus"
	"tinygo.org/x/bluetooth"
)

var (
	dev     = flag.String("dev", "", "")
	scan    = flag.Bool("scan", false, "")
	logfile = flag.String("log", "", "")
)
var adapter = bluetooth.DefaultAdapter

func logLine(s string) error {
	if *logfile != "" {
		f, err := os.OpenFile(*logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err = f.WriteString(s + "\n"); err != nil {
			return err
		}
		return f.Close()
	}
	fmt.Println(s)
	return nil
}

func main() {
	flag.Parse()
	if flag.NArg() > 0 {
		log.Fatalf("Trailing args on command line: %q", flag.Args())
	}

	serviceUUID, err := bluetooth.ParseUUID("b42e1c08-ade7-11e4-89d3-123b93f75cba")
	if err != nil {
		log.Fatalf("Parsing service UUID: %v", err)
	}

	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())

	// Start scanning.
	if *scan {
		println("Scanning...")
		var ress []bluetooth.ScanResult
		go func() {
			time.Sleep(3 * time.Second)
			adapter.StopScan()
		}()
		if err := adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
			if !device.HasServiceUUID(serviceUUID) {
				return
			}
			log.Info("Found device:", device.Address.String(), device.RSSI, device.LocalName(), device.AdvertisementPayload)
			ress = append(ress, device)
		}); err != nil {
			log.Fatal("Scanning: %v", err)
		}
		for _, device := range ress {
			func() {
				addr := device.Address
				d, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
				if err != nil {
					log.Warningf("Connecting to device %q: %v", addr.String(), err)
					return
				}
				defer d.Disconnect()

				servs, err := d.DiscoverServices([]bluetooth.UUID{bluetooth.ServiceUUIDDeviceInformation})
				if err != nil {
					log.Warningf("Discover services on %q: %v", addr.String(), err)
					return
				}
				var model string
				var serial string
				for _, s := range servs { // should be exactly 1
					chs, err := s.DiscoverCharacteristics(nil) // No filter because apparently that doesn't work.
					if err != nil {
						log.Warningf("Listing characteristics on %q: %v", addr.String(), err)
						return
					}
					for _, ch := range chs {
						var dst *string
						switch ch.UUID().String() {
						case bluetooth.CharacteristicUUIDSerialNumberString.String():
							dst = &serial
						case bluetooth.CharacteristicUUIDModelNumberString.String():
							dst = &model
						default:
							continue
						}
						b := make([]byte, 128, 128)
						if n, err := ch.Read(b); err != nil {
							log.Error(err)
							continue
						} else {
							b = b[:n]
						}
						*dst = string(b)
					}
					if model != "" && serial != "" {
						break
					}
				}
				fmt.Printf("%s %4d %20s %s%s\n", device.Address.String(), device.RSSI, device.LocalName(), model, serial)
			}()
		}
		return
	}

	// Perform a scan, because otherwise apparently connection doesn't work.
	go func() {
		time.Sleep(1 * time.Second)
		adapter.StopScan()
	}()
	if err := adapter.Scan(func(_ *bluetooth.Adapter, _ bluetooth.ScanResult) {
		// Don't care about scan results.
	}); err != nil {
		log.Fatal("Scanning: %v", err)
	}

	// Perform actual connect.
	mac, err := bluetooth.ParseMAC(*dev)
	if err != nil {
		log.Fatalf("Parsing bluetooth %q: %v", *dev, err)
	}
	addr := bluetooth.Address{
		MACAddress: bluetooth.MACAddress{
			MAC: mac,
		},
	}
	d, err := adapter.Connect(addr, bluetooth.ConnectionParams{})
	if err != nil {
		log.Fatalf("Connecting to %v: %v", addr, err)
	}
	defer func() {
		if err := d.Disconnect(); err != nil {
			log.Errorf("Disconnect: %v", err)
		}
	}()

	servs, err := d.DiscoverServices([]bluetooth.UUID{serviceUUID, bluetooth.ServiceUUIDDeviceInformation})
	//servs, err := d.DiscoverServices(nil)
	if err != nil {
		log.Fatalf("Discover %q: %v", serviceUUID, err)
	}

	// Get serial.
	serial := ""
	model := ""
	for _, s := range servs {
		chs, err := s.DiscoverCharacteristics(nil)
		//chs, err := s.DiscoverCharacteristics([]bluetooth.UUID{bluetooth.CharacteristicUUIDSerialNumberString})
		if err != nil {
			log.Fatal("Discover serial: ", err)
		}
		for _, ch := range chs {

			switch ch.String() {
			case bluetooth.CharacteristicUUIDModelNumberString.String():
				t, err := ch2s(ch)
				if err != nil {
					log.Error(err)
				} else {
					model = t
				}
			case bluetooth.CharacteristicUUIDSerialNumberString.String():
				t, err := ch2s(ch)
				if err != nil {
					log.Error(err)
				} else {
					serial = t
				}
				break
			}
		}
	}
	for _, s := range servs {
		// Read all available characteristics data.
		if false {
			// TODO:
			// CharacteristicUUIDModelNumberString is model number2 "2930"
			// ServiceUUIDFirmwareRevision "G-BLE-1.5.3-master+0"
			// CharacteristicUUIDSystemID a binary string
			// CharacteristicUUIDManufacturerNameString "Airthings AS"
			// CharacteristicUUIDHardwareRevisionString "REV A"
			chs, err := s.DiscoverCharacteristics(nil)
			//chs, err := s.DiscoverCharacteristics([]bluetooth.UUID{bluetooth.CharacteristicUUIDSerialNumberString})
			if err != nil {
				log.Fatal("Serial char: ", err)
			}
			//CharacteristicUUIDSerialNumberString
			//> char-read-hnd 0x29
			//Characteristic value/descriptor: 02 2a 00 25 2a

			for _, ch := range chs {
				if ch.String() != bluetooth.CharacteristicUUIDSerialNumberString.String() {
					//continue
				}
				fmt.Printf("Serv %q, Char %q\n", s.UUID(), ch.UUID())
				b := make([]byte, 128, 128)
				if n, err := ch.Read(b); err != nil {
					log.Error(err)
					continue
				} else {
					b = b[:n]
				}
				fmt.Printf("Read data: %q\n", string(b))
			}
		}
		//fmt.Println("A> ", s.UUID().String())
		//fmt.Println("B> ", uu.String())
		if s.UUID().String() != serviceUUID.String() {
			continue
		}

		//fmt.Println("Found service")
		cuu, err := bluetooth.ParseUUID("b42e2a68-ade7-11e4-89d3-123b93f75cba")
		if err != nil {
			log.Fatalf("Parsing UUID: %v", err)
		}
		chs, err := s.DiscoverCharacteristics([]bluetooth.UUID{cuu})
		if err != nil {
			log.Fatal(err)
		}
		for _, ch := range chs {
			//fmt.Println("…", ch)
			if ch.UUID().String() != cuu.String() {
				continue
			}
			//fmt.Println("Found character")
			var b []byte
			b = make([]byte, 128, 128)
			if n, err := ch.Read(b); err != nil {
				log.Fatal(err)
			} else {
				b = b[:n]
			}
			version := int(b[0])
			if version != 1 {
				log.Warningf("Unknown version %d, only support version 1", version)
			}
			//
			humid := int(b[1])
			var temp int16
			var radonLong uint16
			var radonShort uint16
			var pressure uint16
			var co2 uint16
			var voc uint16
			buf := bytes.NewReader(b[4:])
			if err := binary.Read(buf, binary.LittleEndian, &radonShort); err != nil {
				log.Fatal(err)
			}
			if err := binary.Read(buf, binary.LittleEndian, &radonLong); err != nil {
				log.Fatal(err)
			}
			if err := binary.Read(buf, binary.LittleEndian, &temp); err != nil {
				log.Fatal(err)
			}
			if err := binary.Read(buf, binary.LittleEndian, &pressure); err != nil {
				log.Fatal(err)
			}
			if err := binary.Read(buf, binary.LittleEndian, &co2); err != nil {
				log.Fatal(err)
			}
			if err := binary.Read(buf, binary.LittleEndian, &voc); err != nil {
				log.Fatal(err)
			}
			if false {
				fmt.Printf("Humidity: %f\n", float64(humid)/2.0)
				fmt.Printf("Radon: %d %d\n", radonShort, radonLong)
				fmt.Printf("Temperature: %f\n", float64(temp)/100.0)
				fmt.Printf("Pressure: %f\n", float64(pressure)/50.0)
				fmt.Printf("CO2: %d\n", co2)
				fmt.Printf("VOC: %d\n", voc)
			} else {
				type Data struct {
					Serial      string  `json:"serial"`
					Humidity    float32 `json:"humidity"`
					RadonShort  int     `json:"radon_short"`
					RadonLong   int     `json:"radon_long"`
					Temperature float32 `json:"temperature"`
					Pressure    float32 `json:"pressure"`
					CO2         int     `json:"co2"`
					VOC         int     `json:"voc"`
				}
				data := Data{
					Serial:      model + serial,
					Humidity:    float32(humid) / 2.0,
					RadonShort:  int(radonShort),
					RadonLong:   int(radonLong),
					Temperature: float32(temp) / 100.0,
					Pressure:    float32(pressure) / 50.0,
					CO2:         int(co2),
					VOC:         int(voc),
				}
				b, err := json.Marshal(&data)
				if err != nil {
					log.Fatal(err)
				}
				if err := logLine(string(b)); err != nil {
					log.Fatalf("Logging: %v", err)
				}
			}
		}
	}

}

func onPeriphConnected(p gatt.Peripheral, err error) {
	log.Printf("Peripheral connected\n")
}
func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}
func ch2s(ch bluetooth.DeviceCharacteristic) (string, error) {
	b := make([]byte, 128, 128)
	n, err := ch.Read(b)
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", fmt.Errorf("serial/model empty string")
	}
	return string(b[:n]), nil
}
