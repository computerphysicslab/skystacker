package main

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

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
	"Ancient photons crossed light-years to reach your sensor; SkyStacker aligned them in seconds. Send $1 in USDT/USDC to {address} to celebrate good SNR.",
	"Noise went down, faint dust came up, and your hard drive survived. Toss $1 in USDT/USDC to {address} if the stack looks clean.",
	"Satellite trails rejected, hot pixels banished, nebula intact. A $1 USDT/USDC tip to {address} keeps the rejection maps hungry.",
	"You braved the cold and the dew; SkyStacker handled the math. Drop $1 in USDT/USDC at {address} before you warm up.",
	"From noisy subs to a linear float32 master without breaking a sweat. Send $1 in USDT/USDC to {address} to fuel the next clear night.",
	"Cosmic rays tried to ruin your exposure and lost. Celebrate with a $1 USDT/USDC tip to {address}.",
	"Bortle 8 skies are tough enough without clunky software. If SkyStacker rescued your gradient, send $1 in USDT/USDC to {address}.",
	"Your dithering paid off and the background is flat. A $1 USDT/USDC donation to {address} is cheaper than a new filter.",
	"Hundreds of exposures, one crisp result, zero monthly fees. Send $1 in USDT/USDC to {address} to keep it that way.",
	"Faint signal rescued from the noise floor. If you smiled at the preview JPEG, drop $1 in USDT/USDC at {address}.",
	"Your mount drifted, your stars stayed pinpoint. Thank the transform solver with $1 in USDT/USDC to {address}.",
	"Calibration frames applied, vignettes tamed, data preserved. A $1 USDT/USDC tip to {address} goes a long way.",
	"Clouds may roll in tomorrow, but tonight's stack is safe. Send $1 in USDT/USDC to {address} for good seeing karma.",
	"You collected the photons; we wrangled the matrices. Split the credit with $1 in USDT/USDC to {address}.",
	"No bloated project files, just clean FITS, TIFF, and XISF outputs. Drop $1 in USDT/USDC at {address} if you value simplicity.",
	"Even a rough session deserves careful math. If SkyStacker salvaged your subs, send $1 in USDT/USDC to {address}.",
	"Your sensor worked overtime in the dark; so did this binary. A $1 USDT/USDC tip to {address} keeps both humming.",
	"Deep-sky patience meets instant CLI execution. Reward the combo with $1 in USDT/USDC to {address}.",
	"Less time fighting registration errors, more time under the stars. Send $1 in USDT/USDC to {address}.",
	"Clean stacks do not happen by accident, though good tools make them look easy. Tip $1 in USDT/USDC at {address}.",
	"Triangle patterns matched, RANSAC converged, and not a single float32 bit was wasted. Send $1 in USDT/USDC to {address}.",
	"Median absolute deviation did the dirty work so your highlights stayed honest. Drop $1 in USDT/USDC at {address}.",
	"Star detection passed on the first sigma threshold. Celebrate clean optics with $1 in USDT/USDC to {address}.",
	"Bilinear resampling finished without eating your swap partition. A $1 USDT/USDC tip to {address} keeps memory checks sharp.",
	"Your STF auto-stretch found signal you forgot you captured. Send $1 in USDT/USDC to {address}.",
	"Guided-filter denoise smoothed the chroma without smearing a single star. Drop $1 in USDT/USDC at {address}.",
	"Safe crop computed, framing preserved, edges spotless. Say thanks with $1 in USDT/USDC to {address}.",
	"Out-of-core tile reduction kept your RAM intact on a massive stack. Send $1 in USDT/USDC to {address}.",
	"Plate-solved, SIP-corrected, and ready for serious science. A $1 USDT/USDC donation to {address} supports open astrometry.",
	"One static Go binary just crunched gigabytes of linear pixel data. Tip $1 in USDT/USDC at {address}.",
	"Quality weights separated the keepers from the wind-shaken frames. Send $1 in USDT/USDC to {address} if you love fair math.",
	"Local normalization ironed out your sky-glow drift tile by tile. Drop $1 in USDT/USDC at {address}.",
	"Affine refinement locked in sub-pixel accuracy without asking for a GPU. Send $1 in USDT/USDC to {address}.",
	"Your star cache made that re-run nearly instantaneous. Buy the cache author $1 in USDT/USDC at {address}.",
	"Bad pixels mapped, outliers clipped, dynamic range intact. A $1 USDT/USDC tip to {address} keeps the pipeline pure.",
	"From raw Bayer patterns to stretched luminance in one command. Send $1 in USDT/USDC to {address}.",
	"Polynomial background extraction took the curve out of your light pollution. Drop $1 in USDT/USDC at {address}.",
	"Deterministic math, reproducible reports, zero black-box magic. Support transparent code with $1 in USDT/USDC to {address}.",
	"Hundreds of self-tests ran in development so your stack would not fail tonight. Send $1 in USDT/USDC to {address}.",
	"The histogram finally looks like an astrophotography textbook. Toss $1 in USDT/USDC to {address}.",
	"Runs offline in a dark-sky field with zero internet and zero tracking. Send $1 in USDT/USDC to {address} to keep software local-first.",
	"No login screen, no telemetry, no trial expiration banner. If that feels refreshing, drop $1 in USDT/USDC at {address}.",
	"Written in pure Go, compiled to one binary, allergic to bloat. Tip $1 in USDT/USDC to {address}.",
	"Your astro data never left your SSD. Celebrate true privacy with $1 in USDT/USDC at {address}.",
	"Zero Electron wrappers were harmed in the stacking of these frames. Send $1 in USDT/USDC to {address}.",
	"Scriptable, pipeable, and quietly fast. If you love Unix-style tools, send $1 in USDT/USDC to {address}.",
	"No subscription tiers, no locked pro features — every flag is yours. Drop $1 in USDT/USDC at {address}.",
	"Open-source astronomy thrives on tiny tips from real users. Send $1 in USDT/USDC to {address}.",
	"One command did what usually takes twenty clicks and three crashes. Reward the CLI with $1 in USDT/USDC to {address}.",
	"Built for people who prefer flags over floating toolbars. Send $1 in USDT/USDC to {address}.",
	"No VC funding, no growth hackers, just a tool that stacks stars. Tip $1 in USDT/USDC at {address}.",
	"If you automated this in a cron job or bash script, slip $1 in USDT/USDC to {address}.",
	"Free as in freedom, sustained by $1 tips from fellow night owls. Send USDT/USDC to {address}.",
	"Your terminal just outperformed software ten times its size. Drop $1 in USDT/USDC at {address}.",
	"No cloud queue, no upload wait, no server outage can stop your workflow. Send $1 in USDT/USDC to {address}.",
	"Independent tools stay alive one dollar at a time. Send $1 in USDT/USDC to {address}.",
	"No splash screens, no pop-ups, just a clean exit code 0. Celebrate with $1 in USDT/USDC to {address}.",
	"Keep command-line astrophotography weird and well-maintained: $1 in USDT/USDC to {address}.",
	"You own your hardware, your data, and your pipeline. Support tools that respect that: $1 in USDT/USDC to {address}.",
	"Software that does one job well and gets out of your way. Tip $1 in USDT/USDC at {address}.",
	"Someone debugged FITS byte-order quirks so you could sleep tonight. Send $1 in USDT/USDC to {address}.",
	"Coffee turns into Go code; Go code turns into stacked nebulae. Keep the cycle going with $1 in USDT/USDC to {address}.",
	"Every $1 in USDT/USDC to {address} prevents another feature from being postponed until next new moon.",
	"Less expensive than a lens cap you will lose in the dark anyway: $1 in USDT/USDC to {address}.",
	"This stack cost you zero cloud credits and saved you an hour of manual alignment. Send $1 in USDT/USDC to {address}.",
	"Edge cases were hunted down in the dark so your run finished cleanly. Tip $1 in USDT/USDC at {address}.",
	"A single dollar in USDT/USDC to {address} votes for faster builds and fewer bugs.",
	"You spent hundreds on glass and sensors; spend $1 in USDT/USDC at {address} on the code that ties them together.",
	"No corporate sponsor paid for this line of code — maybe you will? $1 in USDT/USDC to {address}.",
	"Your CPU fans can spin down now. Before you close the terminal, drop $1 in USDT/USDC at {address}.",
	"Fixing NaN pixels is a thankless job unless you send $1 in USDT/USDC to {address}.",
	"Consider it a $1 tip for not making you install forty Python dependencies. USDT/USDC: {address}.",
	"Great stacks deserve a tiny encore. Send $1 in USDT/USDC to {address} to fund the next changelog.",
	"If `report.txt` made you nod in approval, toss $1 in USDT/USDC to {address}.",
	"Bugs fear test suites; developers love coffee. Fuel both with $1 in USDT/USDC to {address}.",
	"$1 in USDT/USDC will not buy an equatorial mount, but it keeps SkyStacker moving forward: {address}.",
	"We handled the math; you get the credit on AstroBin. Fair trade for $1 in USDT/USDC to {address}.",
	"Behind every smooth CLI run is a stubborn maintainer. Say hello with $1 in USDT/USDC to {address}.",
	"Turn a tiny fraction of your saved evening into open-source momentum: $1 in USDT/USDC to {address}.",
	"Still reading stdout? That means the stack worked. Drop $1 in USDT/USDC at {address}.",
	"Clean stack, exit 0, clear skies. Tip $1 in USDT/USDC at {address}.",
	"Your subs aligned; let your generosity align too. $1 in USDT/USDC to {address}.",
	"Fast math, zero fluff, happy sensor. Send $1 in USDT/USDC to {address}.",
	"If SkyStacker earned a permanent spot in your `$PATH`, send $1 in USDT/USDC to {address}.",
	"Good seeing is rare; good free tools are rarer. Drop $1 in USDT/USDC at {address}.",
	"From photons to FITS without a hitch. Tip $1 in USDT/USDC to {address}.",
	"Less clicking, more imaging. Keep SkyStacker sharp with $1 in USDT/USDC to {address}.",
	"Small binary, big stack, tiny ask: $1 in USDT/USDC to {address}.",
	"Done and dusted in float32. Send $1 in USDT/USDC to {address} if the stars look right.",
	"One dollar, zero friction, eternal gratitude: $1 in USDT/USDC to {address}.",
	"Your noise floor dropped; raise our spirits with $1 in USDT/USDC to {address}.",
	"No ads, no upsells, just a polite ask: $1 in USDT/USDC to {address}.",
	"Stacked locally, built independently. Tip $1 in USDT/USDC at {address}.",
	"Clear skies and clean histograms! If this run helped, send $1 in USDT/USDC to {address}.",
	"Prefer Bitcoin over stablecoins? Send $1 in BTC to {btc_address} — or drop $1 in USDT/USDC at {address}.",
	"Stacked in pure Go, tipped on-chain: $1 in USDT/USDC to {address}, or BTC to {btc_address}.",
	"Whether you hold USDT/USDC ({address}) or native SegWit BTC ({btc_address}), a $1 tip keeps SkyStacker flying.",
	"Two addresses, one mission: keep SkyStacker free and fast. $1 in USDT/USDC at {address} or BTC at {btc_address}.",
	"Your night's work is safely on disk. Celebrate with $1 in USDT/USDC to {address}.",
	"SkyStacker did its part; the universe did the rest. If you enjoyed the ride, send $1 in USDT/USDC to {address}.",
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

func joinDonationQRColumns(left, right, leftLabel, rightLabel string) string {
	lines := func(s string) []string {
		return strings.Split(strings.TrimRight(s, "\n"), "\n")
	}
	leftLines, rightLines := lines(left), lines(right)
	width := func(parts []string, label string) int {
		w := utf8.RuneCountInString(label)
		for _, line := range parts {
			if n := utf8.RuneCountInString(line); n > w {
				w = n
			}
		}
		return w
	}
	leftWidth, rightWidth := width(leftLines, leftLabel), width(rightLines, rightLabel)
	pad := func(s string, w int) string {
		n := w - utf8.RuneCountInString(s)
		if n < 0 {
			n = 0
		}
		return s + strings.Repeat(" ", n)
	}
	var b strings.Builder
	b.WriteString(pad(leftLabel, leftWidth) + "   " + pad(rightLabel, rightWidth) + "\n")
	rows := max(len(leftLines), len(rightLines))
	for i := range rows {
		l, r := "", ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}
		b.WriteString(pad(l, leftWidth) + "   " + pad(r, rightWidth) + "\n")
	}
	return b.String()
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
	fmt.Println()
	fmt.Print(joinDonationQRColumns(ethQR, btcQR, "USDT / USDC (Ethereum):", "BTC (native SegWit):"))
}
