package main

import (
	"fmt"

	"github.com/morikuni/aec"
)

func main() {
	fmt.Printf("%13s %s\n", "black", aec.BlackB.Apply("  "))
	fmt.Printf("%13s %s\n", "red", aec.RedB.Apply("  "))
	fmt.Printf("%13s %s\n", "green", aec.GreenB.Apply("  "))
	fmt.Printf("%13s %s\n", "yellow", aec.YellowB.Apply("  "))
	fmt.Printf("%13s %s\n", "blue", aec.BlueB.Apply("  "))
	fmt.Printf("%13s %s\n", "magenta", aec.MagentaB.Apply("  "))
	fmt.Printf("%13s %s\n", "cyan", aec.CyanB.Apply("  "))
	fmt.Printf("%13s %s\n", "white", aec.WhiteB.Apply("  "))
	fmt.Println()

	fmt.Printf("%13s %s\n", "light-black", aec.LightBlackB.Apply("  "))
	fmt.Printf("%13s %s\n", "light-red", aec.LightRedB.Apply("  "))
	fmt.Printf("%13s %s\n", "light-green", aec.LightGreenB.Apply("  "))
	fmt.Printf("%13s %s\n", "light-yellow", aec.LightYellowB.Apply("  "))
	fmt.Printf("%13s %s\n", "light-blue", aec.LightBlueB.Apply("  "))
	fmt.Printf("%13s %s\n", "light-magenta", aec.LightMagentaB.Apply("  "))
	fmt.Printf("%13s %s\n", "light-cyan", aec.LightCyanB.Apply("  "))
	fmt.Printf("%13s %s\n", "light-white", aec.LightWhiteB.Apply("  "))
	fmt.Println()

	for r := uint8(0); r < 6; r++ {
		for g := uint8(0); g < 6; g++ {
			for b := uint8(0); b < 6; b++ {
				ansi := aec.Color8BitB(aec.NewRGB8Bit(r*43, g*43, b*43))
				fmt.Printf("%02x-%02x-%02x %s ", r*43, g*43, b*43, ansi.Apply("  "))
			}
			fmt.Println()
		}
		fmt.Println()
	}

	fmt.Println(aec.Bold.Apply("bold"))
	fmt.Println(aec.Faint.Apply("faint"))
	fmt.Println(aec.Italic.Apply("italic"))
	fmt.Println(aec.Underline.Apply("underline"))
	fmt.Println(aec.BlinkSlow.Apply("blink-slow"))
	fmt.Println(aec.BlinkRapid.Apply("blink-rapid"))
	fmt.Println(aec.Inverse.Apply("inverse"))
	fmt.Println(aec.Conceal.Apply("conceal"))
	fmt.Println(aec.CrossOut.Apply("cross-out"))
	fmt.Println(aec.Frame.Apply("frame"))
	fmt.Println(aec.Encircle.Apply("encircle"))
	fmt.Println(aec.Overline.Apply("overline"))
}
