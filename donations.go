package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

const donationEthAddress = "0xE5f68950D479fAB12797dABbe5a4B0d88eC7A722"
const donationBTCAddress = "bc1qz4zvf42l3fu4axp0nd85fp2jr0gnavrr2v6xsj"

var donationMessages = []string{
	"SkyStacker finished the heavy lifting. If it saved you an evening, send $1 in USDT/USDC to {address} — tiny gestures move mountains.",
	"Your pixels are aligned and your night is saved. A $1 USDT/USDC tip to {address} keeps this CLI stubbornly independent.",
	"No GUI, no cloud, no nonsense — just a clean stack. If that feels rare, drop $1 in USDT/USDC at {address}.",
	"The stars cooperated. Now it is your move: $1 in USDT/USDC to {address} buys another late-night debugging session.",
	"Your stack survived sigma clipping. A $1 USDT/USDC donation to {address} helps the next one survive too.",
	"Because nobody pays for what they do not notice: $1 in USDT/USDC to {address} keeps SkyStacker awake.",
	"Your calibration was recoverable; our motivation is not infinite. $1 USDT/USDC to {address}.",
	"Ten subs, one stubborn algorithm, zero ads. If that is your vibe, $1 in USDT/USDC to {address}.",
	"The data says thank you. We say: $1 in USDT/USDC to {address} and the next release arrives faster.",
	"Bitcoin purist? {btc_address} also accepts a $1 tip. Stablecoin crowd: USDT/USDC on Ethereum at {address}. Either way, thank you.",
}

func donationFooterFor(seed int64) string {
	idx := int((seed^(seed>>17))%int64(len(donationMessages))+int64(len(donationMessages))) % len(donationMessages)
	msg := donationMessages[idx]
	msg = strings.ReplaceAll(msg, "{address}", donationEthAddress)
	msg = strings.ReplaceAll(msg, "{btc_address}", donationBTCAddress)
	return msg
}

func renderDonationQR(text string) (string, error) {
	q, err := qrcode.New(text, qrcode.Low)
	if err != nil {
		return "", err
	}
	bitmap := q.Bitmap()
	var b strings.Builder
	for y := 0; y < len(bitmap); y += 2 {
		for x := 0; x < len(bitmap[y]); x++ {
			upper := bitmap[y][x]
			lower := false
			if y+1 < len(bitmap) {
				lower = bitmap[y+1][x]
			}
			switch {
			case upper && lower:
				b.WriteString("██")
			case upper:
				b.WriteString("▀ ")
			case lower:
				b.WriteString("▄ ")
			default:
				b.WriteString("  ")
			}
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func printDonationFooter(enabled, showQR bool) {
	if !enabled || os.Getenv("SKYSTACKER_NO_DONATION") != "" {
		return
	}
	fmt.Println()
	fmt.Println(donationFooterFor(time.Now().UnixNano() ^ int64(os.Getpid())))
	if !showQR {
		return
	}
	ethQR, err := renderDonationQR(donationEthAddress)
	if err != nil {
		return
	}
	btcQR, err := renderDonationQR(donationBTCAddress)
	if err != nil {
		return
	}
	fmt.Println("\nUSDT / USDC (Ethereum):")
	fmt.Print(ethQR)
	fmt.Println("BTC (native SegWit):")
	fmt.Print(btcQR)
}
