package main

import (
	"fmt"
	"time"
	"github.com/westphae/goflying/bmp280"
)

const (
	RETRIES = 10
	FREQ = 0x03
)

func main() {
	freq := 500
	if FREQ==1 {
		freq = 62500
	} else if FREQ > 1 {
		freq = int(uint(4000) >> uint(7-FREQ))*1000
	}
	clock := time.NewTicker(time.Duration(freq) * time.Microsecond)
	var (
		bmp      *bmp280.BMP280
		cur      *bmp280.BMPData
		err      error
	)

	for i:=0; i<RETRIES; i++ {
		bmp, err = bmp280.NewBMP280(0x03, FREQ, 0x04, 0x05, 0x05)

		fmt.Printf("DigT1: %d, DigT2: %d, DigT3: %d\n", bmp.DigT1, bmp.DigT2, bmp.DigT3)
		fmt.Printf("DigP1: %d, DigP2: %d, DigP3: %d\n", bmp.DigP1, bmp.DigP2, bmp.DigP3)
		fmt.Printf("DigP4: %d, DigP5: %d, DigP6: %d\n", bmp.DigP4, bmp.DigP5, bmp.DigP6)
		fmt.Printf("DigP7: %d, DigP8: %d, DigP9: %d\n", bmp.DigP7, bmp.DigP8, bmp.DigP9)

		if err != nil {
			fmt.Printf("Error initializing BMP280, attempt %d of 10, %s\n", i, err)
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
	if err != nil {
		fmt.Println("Error: couldn't initialize BMP280")
		return
	}
	bmp.Close()
	fmt.Println("BMP280 initialized successfully")

	for {
		<-clock.C

		cur = <-bmp.C
		fmt.Printf("\nTime:   %v\n", cur.T)
		fmt.Printf("Temperature: %3.1f\n", cur.Temperature)
		fmt.Printf("Pressure: %4.1f\n", cur.Pressure)
		fmt.Printf("Altitude: %5.0f\n", cur.Altitude)
	}
}