// SkyStacker v0.5.0 — single-file astrophotography stacker.
//
// Build:  go build -o skystacker main.go
// Test:   ./skystacker --selftest
// Run:    ./skystacker [flags] [input-directory]
//
// Stacking pipeline:
//   - Discovery and decode of JPG, RAW (CR2/CR3 via dcraw +
//     ImageMagick) and FITS (via ImageMagick). Optional calibration with
//     dark, flat and bias frames; --calib-root auto-detects subfolders.
//   - High-pass star detection with sub-pixel centroiding: stars are
//     found even on bright nebula backgrounds.
//   - Reference-frame selection by star count; triangle-invariant star
//     matching; RANSAC similarity estimation (rotation + uniform scale
//     + translation) with optional full-affine refinement; bilinear
//     sub-pixel resampling; safe crop to the largest fully-valid
//     region.
//   - Stacking methods: mean (streaming), median and sigma (tiled,
//     out-of-core, MAD-based rejection, per-pixel weight and rejection
//     maps). Median/sigma frames are spilled to disk so peak memory
//     stays bounded regardless of session size.
//
// Post-processing pipeline (each output is independently inspectable):
//   step 1  background removal  (polynomial degree 1-3)
//   step 2  star removal        (high-pass + median annulus replacement)
//   step 3  black-point shift   (intermediate; not written to disk)
//   step 4  PixInsight-style STF/MTF auto-stretch
//   step 5  guided-filter denoise (iterative, 1-10 passes)
//   step 6  background-darkening curves (crush shadows, keep highlights)
//   step 7  HDR blend (step 6 + linear background-removed frame)
//   step 8  weighted mix (4 mild variants of step 7 + step 1);
//           optional diffraction-spike overlay (on by default)
//
// Diagnostics:
//   - Per-frame stars / inliers / scale / rotation / offset /
//     quality / bounding box, plus a crop explanation naming the frames
//     that constrain the crop.
//   - Post-processing outputs are saved as JPEGs plus a machine-readable
//     report.txt.
//
// The binary is a single file so it can be copied, audited and compiled
// without a dependency tree. Only the Go standard library is used at
// build time; external tools (dcraw, ImageMagick) are only needed at
// run time for RAW and FITS decoding.

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var version = "0.5.0"

// repoURL is printed in the startup banner and in --version output.
const repoURL = "https://github.com/computerphysicslab/skystacker"

// buildDate is injected at build time via:
//
//	go build -ldflags "-X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o skystacker main.go
//
// When empty, the VCS commit time embedded by Go is used.
var buildDate = ""

// =========================================================================
// CLI
// =========================================================================

type cliOptions struct {
	Clean                bool
	InputDir             string
	OutputDir            string
	Workers              int
	Recursive            bool
	Overwrite            bool
	Version              bool
	Help                 bool
	SelfTest             bool
	Register             bool
	Method               string
	MaskBG               bool
	DarkFile             string
	FlatFile             string
	BiasFile             string
	CalibRoot            string
	PostProcess          bool
	StackOnly            bool
	PostOnly             bool
	NoColorCalib         bool
	NoSharpen            bool
	BGDegree             int
	StretchFactor        float64
	SharpenAmount        float64
	SharpenRadius        float64
	HDRAmount            float64
	StarRadius           int
	StarSigma            float64
	BgPercentile         float64
	NoAutoStretch        bool
	STFShadowsClip       float64
	STFTargetBG          float64
	STFLinked            bool
	NoHDRBlend           bool
	HDRBlendFactor       float64
	NoHDRMix             bool
	NoDenoise            bool
	DenoiseStrength      float64
	DenoiseRadius        int
	DenoiseChroma        float64
	NoCurves             bool
	CurvesBlack          float64
	CurvesGamma          float64
	AddSpikes            bool
	SpikesPercent        float64
	SpikesLength         float64
	SpikesIntensity      float64
	SpikesArms           int
	SpikesSoftness       float64
	SpikesMinRatio       float64
	SpikesLengthSoftness float64
	SpikesLengthMinRatio float64
	SpikesMinCount       int
	NoExif               bool
	SigmaClipThreshold   float64
	SigmaClipIters       int
	TileSize             int
	Scale                int
	Force                bool
	Normalize            bool
	LocalNormalize       bool
	LNTileSize           int

	CreateTIFF  bool
	PostCA      bool
	PostColor   bool
	PostFlatten bool
	PostSharpen bool
	PostHDR     bool
	PostStretch bool

	NoCache     bool
	CacheDir    string
	TestDebayer string
}

func parseArgs(args []string) (*cliOptions, error) {
	fs := flag.NewFlagSet("skystacker", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	o := &cliOptions{}
	var noRegister bool
	fs.StringVar(&o.OutputDir, "output", "", "output directory (default: <input>/0_skystacker_results)")
	fs.IntVar(&o.Workers, "workers", runtime.NumCPU(), "maximum concurrent workers")
	fs.BoolVar(&o.Recursive, "recursive", false, "search subdirectories")
	fs.BoolVar(&o.Overwrite, "overwrite", false, "permit replacing files in the output directory")
	fs.BoolVar(&o.Version, "version", false, "print version and exit")
	fs.BoolVar(&o.SelfTest, "selftest", false, "run internal self-tests and exit")
	fs.StringVar(&o.Method, "method", "sigma", "stacking method: mean, median, sigma")
	fs.BoolVar(&o.MaskBG, "mask-bg", false, "mask bright objects during background extraction")
	fs.StringVar(&o.DarkFile, "dark", "", "path to dark frame for calibration")
	fs.StringVar(&o.FlatFile, "flat", "", "path to flat frame for calibration")
	fs.StringVar(&o.BiasFile, "bias", "", "path to bias frame or folder for calibration")
	fs.StringVar(&o.CalibRoot, "calib-root", "", "auto-detect Bias/Darks/Flats inside this root folder")
	fs.BoolVar(&noRegister, "no-register", false, "disable star-based registration")
	var noSpikes bool
	fs.BoolVar(&noSpikes, "no-spikes", false, "disable diffraction spikes (overrides --spikes)")
	var noPostProcess bool
	var stackOnly, postOnly bool
	fs.BoolVar(&noPostProcess, "no-postprocess", false, "alias for --stack-only")
	fs.BoolVar(&stackOnly, "stack-only", false, "run stacking only, skip post-processing")
	fs.BoolVar(&postOnly, "post-only", false, "run post-processing only using existing TIFFs")
	fs.BoolVar(&o.NoColorCalib, "no-color-calib", false, "skip color calibration step")
	fs.BoolVar(&o.NoSharpen, "no-sharpen", false, "skip sharpening step")
	fs.IntVar(&o.BGDegree, "bg-degree", 2, "background polynomial degree (1-3)")
	fs.Float64Var(&o.StretchFactor, "stretch", 10.0, "asinh stretch factor")
	fs.Float64Var(&o.SharpenAmount, "sharpen-amount", 0.8, "unsharp mask amount")
	fs.Float64Var(&o.SharpenRadius, "sharpen-radius", 1.5, "unsharp mask radius in pixels")
	fs.Float64Var(&o.HDRAmount, "hdr-amount", 0.5, "HDR local contrast amount")
	fs.IntVar(&o.StarRadius, "star-radius", 0, "star removal radius in px (0 = auto)")
	fs.Float64Var(&o.StarSigma, "star-sigma", 4.0, "star detection sigma (lower = more aggressive)")
	fs.Float64Var(&o.BgPercentile, "bg-percentile", 10.0, "subtract this percentile of luminance (0 disables); higher = darker background")
	fs.BoolVar(&o.NoAutoStretch, "no-auto-stretch", false, "skip the final PixInsight-style STF/MTF auto-stretch")
	fs.Float64Var(&o.STFShadowsClip, "stf-shadows", -2.8, "STF shadow clip in units of sigma (default -2.8)")
	fs.Float64Var(&o.STFTargetBG, "stf-target", 0.25, "STF target background level (default 0.25)")
	fs.BoolVar(&o.STFLinked, "stf-linked", false, "use linked channels for STF (default unlinked for auto color balance)")
	fs.BoolVar(&o.NoHDRBlend, "no-hdr-blend", false, "skip the final HDR blend step (post_step5_hdr.jpg)")
	fs.Float64Var(&o.HDRBlendFactor, "hdr-blend-factor", 1.0, "HDR blend amount: post_step4 + factor*post_step1 (default 1.0)")
	fs.BoolVar(&o.NoHDRMix, "no-hdr-mix", false, "skip the step-7 weighted mix outputs (post_step7_1..4.jpg)")
	fs.BoolVar(&o.NoDenoise, "no-denoise", false, "skip the final denoise step")
	fs.Float64Var(&o.DenoiseStrength, "denoise-strength", 4.0, "denoise iterations (0=skip, 1=one pass, N=N passes)")
	fs.IntVar(&o.DenoiseRadius, "denoise-radius", 3, "guided-filter denoise radius")
	fs.Float64Var(&o.DenoiseChroma, "denoise-chroma", 1.0, "chroma denoise power: 1=one pass, N=N passes (0 disables, max 10)")
	fs.BoolVar(&o.NoCurves, "no-curves", false, "skip the background-darkening curves step (post_step6_curves.jpg)")
	fs.Float64Var(&o.CurvesBlack, "curves-black", 0.03, "curves: shadow crush point (0-0.5, 0 disables crush)")
	fs.Float64Var(&o.CurvesGamma, "curves-gamma", 1.20, "curves: midtone gamma (>1 darkens, <1 brightens, 1 neutral)")
	fs.BoolVar(&o.AddSpikes, "spikes", true, "add diffraction spikes to bright stars (default: on; --no-spikes to disable)")
	fs.Float64Var(&o.SpikesPercent, "spikes-percent", 0.2, "percent of detected stars to spike (0-50, default 0.2)")
	fs.Float64Var(&o.SpikesLength, "spikes-length", 10.0, "spike arm half-length in pixels (1-5000)")
	fs.Float64Var(&o.SpikesIntensity, "spikes-intensity", 0.3, "peak additive spike brightness (0.1-2.0)")
	fs.IntVar(&o.SpikesArms, "spikes-arms", 4, "number of spike arms: 4=cross, 6=hexagonal (default 4)")
	fs.Float64Var(&o.SpikesSoftness, "spikes-softness", 0.5, "spike intensity falloff exponent: 0.1=flat, 0.5=sqrt, 1.0=linear")
	fs.Float64Var(&o.SpikesMinRatio, "spikes-min-ratio", 0.2, "minimum spike size for the faintest spiked star (0-1, default 0.2)")
	fs.Float64Var(&o.SpikesLengthSoftness, "spikes-length-softness", 1.0, "length falloff exponent: 1.0=proportional, 1.5=super-linear, 2.0=strongly differentiated")
	fs.Float64Var(&o.SpikesLengthMinRatio, "spikes-length-min-ratio", 0.03, "minimum length fraction for the faintest spiked star (0-1, default 0.03)")
	fs.IntVar(&o.SpikesMinCount, "spikes-min-count", 5, "minimum number of stars to spike, floors the percent-based selection (default 5)")
	fs.BoolVar(&o.NoExif, "no-exif", false, "skip EXIF exposure summary in report")
	fs.BoolVar(&o.Clean, "clean", false, "remove _work directory after successful run")
	fs.BoolVar(&o.NoCache, "no-cache", false, "disable star/transform cache (default: cache in <output>/_work/cache)")
	fs.StringVar(&o.CacheDir, "cache-dir", "", "cache directory (default: <output>/_work/cache)")
	fs.StringVar(&o.TestDebayer, "test-debayer", "", "test FITS debayering on one file: writes <base>.jpg and <base>_raw.jpg, then exits")
	fs.Float64Var(&o.SigmaClipThreshold, "sigma-clip", 3.0, "sigma-clipping threshold in MAD units (0 disables rejection)")
	fs.IntVar(&o.SigmaClipIters, "sigma-clip-iters", 2, "number of sigma-clipping iterations")
	fs.IntVar(&o.TileSize, "tile-size", 0, "tile side length in px for median/sigma reduction (0 = auto)")
	fs.IntVar(&o.Scale, "scale", 1, "downscale factor (2 = half size, 4 = quarter size)")
	fs.BoolVar(&o.Force, "force", false, "skip OOM safety check (NOT recommended)")
	fs.BoolVar(&o.Normalize, "normalize", false, "globally normalize each frame sky background to the reference frame (requires registration)")
	fs.BoolVar(&o.LocalNormalize, "local-normalize", false, "per-tile normalization (implies --normalize; requires registration)")
	fs.IntVar(&o.LNTileSize, "ln-tile-size", 256, "tile side length for --local-normalize (64-2048)")
	fs.BoolVar(&o.CreateTIFF, "tiff", false, "write TIFF/stacked_sum.tif")
	fs.BoolVar(&o.PostCA, "post-ca", false, "enable chromatic aberration reduction")
	fs.BoolVar(&o.PostColor, "post-color", false, "enable color calibration")
	fs.BoolVar(&o.PostFlatten, "post-flatten", false, "enable vignette flattening")
	fs.BoolVar(&o.PostSharpen, "post-sharpen", false, "enable unsharp mask")
	fs.BoolVar(&o.PostHDR, "post-hdr", false, "enable HDR local contrast")
	fs.BoolVar(&o.PostStretch, "post-stretch", false, "enable 3-level stretch")

	// Pre-process: move any non-flag arguments to the end so flags can appear anywhere
	// e.g., "skystacker Luminance --method mean" -> "skystacker --method mean Luminance"
	var flagsOnly, positionals []string
	skipNext := false
	for _, arg := range args {
		if skipNext {
			flagsOnly = append(flagsOnly, arg)
			skipNext = false
			continue
		}
		if strings.HasPrefix(arg, "-") {
			flagsOnly = append(flagsOnly, arg)
			// If it's a flag that takes a value (not a bool), skip the next arg
			if arg == "--output" || arg == "--workers" || arg == "--method" || arg == "--dark" || arg == "--flat" || arg == "--bias" || arg == "--calib-root" || arg == "--bg-degree" || arg == "--stretch" || arg == "--sharpen-amount" || arg == "--sharpen-radius" || arg == "--hdr-amount" || arg == "--star-radius" || arg == "--star-sigma" || arg == "--bg-percentile" || arg == "--stf-shadows" || arg == "--stf-target" || arg == "--hdr-blend-factor" || arg == "--denoise-strength" || arg == "--denoise-radius" || arg == "--denoise-chroma" || arg == "--curves-black" || arg == "--curves-gamma" || arg == "--spikes-percent" || arg == "--spikes-length" || arg == "--spikes-intensity" || arg == "--spikes-arms" || arg == "--spikes-softness" || arg == "--spikes-min-ratio" || arg == "--spikes-length-softness" || arg == "--spikes-length-min-ratio" || arg == "--spikes-min-count" || arg == "--sigma-clip" || arg == "--sigma-clip-iters" || arg == "--tile-size" || arg == "--scale" || arg == "--ln-tile-size" || arg == "--cache-dir" {
				skipNext = true
			}
		} else {
			positionals = append(positionals, arg)
		}
	}
	args = append(flagsOnly, positionals...)

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			o.Help = true
			return o, nil
		}
		return nil, err
	}
	o.Register = !noRegister
	if noSpikes {
		o.AddSpikes = false
	}
	o.PostProcess = !noPostProcess && !stackOnly
	if stackOnly && postOnly {
		return nil, fmt.Errorf("--stack-only and --post-only are mutually exclusive")
	}
	o.StackOnly = stackOnly
	o.PostOnly = postOnly
	if o.BGDegree < 1 || o.BGDegree > 3 {
		return nil, fmt.Errorf("--bg-degree must be 1, 2 or 3, got %d", o.BGDegree)
	}
	if o.StretchFactor < 0 {
		return nil, fmt.Errorf("--stretch must be >= 0, got %g", o.StretchFactor)
	}
	if o.SharpenAmount < 0 || o.SharpenAmount > 5 {
		return nil, fmt.Errorf("--sharpen-amount must be in [0, 5], got %g", o.SharpenAmount)
	}
	if o.SharpenRadius <= 0 || o.SharpenRadius > 20 {
		return nil, fmt.Errorf("--sharpen-radius must be in (0, 20], got %g", o.SharpenRadius)
	}
	if o.Scale < 1 {
		return nil, fmt.Errorf("--scale must be >= 1, got %d", o.Scale)
	}
	if o.DenoiseStrength < 0 || o.DenoiseStrength > 10 {
		return nil, fmt.Errorf("--denoise-strength must be in [0, 10], got %g", o.DenoiseStrength)
	}
	if o.DenoiseRadius < 1 || o.DenoiseRadius > 20 {
		return nil, fmt.Errorf("--denoise-radius must be in [1, 20], got %d", o.DenoiseRadius)
	}
	if o.DenoiseChroma < 0 || o.DenoiseChroma > 10 {
		return nil, fmt.Errorf("--denoise-chroma must be in [0, 10], got %g", o.DenoiseChroma)
	}
	if o.CurvesBlack < 0 || o.CurvesBlack >= 0.5 {
		return nil, fmt.Errorf("--curves-black must be in [0, 0.5), got %g", o.CurvesBlack)
	}
	if o.CurvesGamma < 0.1 || o.CurvesGamma > 4.0 {
		return nil, fmt.Errorf("--curves-gamma must be in [0.1, 4.0], got %g", o.CurvesGamma)
	}
	if o.SpikesPercent < 0 || o.SpikesPercent > 50 {
		return nil, fmt.Errorf("--spikes-percent must be in [0, 50], got %g", o.SpikesPercent)
	}
	if o.SpikesLength <= 0 || o.SpikesLength > 5000 {
		return nil, fmt.Errorf("--spikes-length must be in (0, 5000], got %g", o.SpikesLength)
	}
	if o.SpikesIntensity <= 0 || o.SpikesIntensity > 2.0 {
		return nil, fmt.Errorf("--spikes-intensity must be in (0, 2.0], got %g", o.SpikesIntensity)
	}
	if o.SpikesArms < 2 || o.SpikesArms > 12 {
		return nil, fmt.Errorf("--spikes-arms must be in [2, 12], got %d", o.SpikesArms)
	}
	if o.SpikesSoftness <= 0 || o.SpikesSoftness > 2.0 {
		return nil, fmt.Errorf("--spikes-softness must be in (0, 2.0], got %g", o.SpikesSoftness)
	}
	if o.SpikesMinRatio < 0 || o.SpikesMinRatio > 1.0 {
		return nil, fmt.Errorf("--spikes-min-ratio must be in [0, 1.0], got %g", o.SpikesMinRatio)
	}
	if o.SpikesLengthSoftness <= 0 || o.SpikesLengthSoftness > 4.0 {
		return nil, fmt.Errorf("--spikes-length-softness must be in (0, 4.0], got %g", o.SpikesLengthSoftness)
	}
	if o.SpikesLengthMinRatio < 0 || o.SpikesLengthMinRatio > 1.0 {
		return nil, fmt.Errorf("--spikes-length-min-ratio must be in [0, 1.0], got %g", o.SpikesLengthMinRatio)
	}
	if o.SpikesMinCount < 0 || o.SpikesMinCount > 1000 {
		return nil, fmt.Errorf("--spikes-min-count must be in [0, 1000], got %d", o.SpikesMinCount)
	}
	if o.SigmaClipThreshold < 0 || o.SigmaClipThreshold > 10 {
		return nil, fmt.Errorf("--sigma-clip must be in [0, 10], got %g", o.SigmaClipThreshold)
	}
	if o.SigmaClipIters < 1 || o.SigmaClipIters > 10 {
		return nil, fmt.Errorf("--sigma-clip-iters must be in [1, 10], got %d", o.SigmaClipIters)
	}
	if o.TileSize < 0 || o.TileSize > 4096 {
		return nil, fmt.Errorf("--tile-size must be in [0, 4096], got %d", o.TileSize)
	}
	if o.LNTileSize < 64 || o.LNTileSize > 2048 {
		return nil, fmt.Errorf("--ln-tile-size must be in [64, 2048], got %d", o.LNTileSize)
	}
	if o.LocalNormalize {
		o.Normalize = true
	}
	rest := fs.Args()
	if len(rest) > 1 {
		return nil, fmt.Errorf("unexpected arguments: %v", rest[1:])
	}
	if len(rest) == 1 {
		o.InputDir = rest[0]
	} else {
		o.InputDir = "."
	}
	if o.Workers < 1 {
		return nil, fmt.Errorf("--workers must be >= 1, got %d", o.Workers)
	}
	return o, nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: skystacker [flags] [input-directory]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Combine JPG/JPEG astrophotography frames with optional star-based")
	fmt.Fprintln(w, "registration into sum, mean and no-correction-mean images.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Arguments:")
	fmt.Fprintln(w, "  input-directory   directory containing image files (default: .)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Auto-detection: Light/, Dark/, Flat/, Bias/ (or plural forms) subfolders")
	fmt.Fprintln(w, "are discovered automatically; --calib-root or --dark/--flat/--bias override.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --output string   output directory (default: <input>/0_skystacker_results)")
	fmt.Fprintln(w, "  --workers int     maximum concurrent workers (default: NumCPU)")
	fmt.Fprintln(w, "  --recursive       search subdirectories")
	fmt.Fprintln(w, "  --overwrite       permit replacing files in the output directory")
	fmt.Fprintln(w, "  --no-register     disable star registration (v0.2 behaviour)")
	fmt.Fprintln(w, "  --method string   stacking method: mean, median, sigma (default \"mean\")")
	fmt.Fprintln(w, "  --dark string     path to dark frame for calibration")
	fmt.Fprintln(w, "  --flat string     path to flat frame for calibration")
	fmt.Fprintln(w, "  --bias string     path to bias frame for calibration")
	fmt.Fprintln(w, "  --stack-only      run stacking only, skip post-processing")
	fmt.Fprintln(w, "  --post-only       run post-processing only using existing TIFFs")
	fmt.Fprintln(w, "  --no-postprocess  alias for --stack-only")
	fmt.Fprintln(w, "  --no-color-calib  skip color calibration step")
	fmt.Fprintln(w, "  --no-sharpen      skip sharpening step")
	fmt.Fprintln(w, "  --bg-degree N     background polynomial degree (1-3, default 2)")
	fmt.Fprintln(w, "  --stretch F       asinh stretch factor (default 10)")
	fmt.Fprintln(w, "  --sharpen-amount F  unsharp mask amount (default 0.8)")
	fmt.Fprintln(w, "  --sharpen-radius F  unsharp mask radius (default 1.5)")
	fmt.Fprintln(w, "  --no-auto-stretch  skip the final STF/MTF auto-stretch")
	fmt.Fprintln(w, "  --stf-shadows F    STF shadow clip in sigma units (default -2.8)")
	fmt.Fprintln(w, "  --stf-target F     STF target background level (default 0.25)")
	fmt.Fprintln(w, "  --stf-linked       use linked channels for STF (default: unlinked)")
	fmt.Fprintln(w, "  --no-hdr-blend    skip the final HDR blend (post_step5_hdr.jpg)")
	fmt.Fprintln(w, "  --hdr-blend-factor F  HDR blend amount: post_step4 + F*post_step1 (default 1.0)")
	fmt.Fprintln(w, "  --no-hdr-mix     skip the step-7 weighted mix (post_step7_1..4.jpg)")
	fmt.Fprintln(w, "  --no-denoise      skip the final denoise step (post_step5_denoised.jpg)")
	fmt.Fprintln(w, "  --denoise-strength F  denoise iterations (0=skip, 1=one pass, N=N passes, max 10)")
	fmt.Fprintln(w, "  --denoise-radius N    guided-filter denoise radius in px (1-20, default 3)")
	fmt.Fprintln(w, "  --denoise-chroma F    chroma denoise power: 1=one pass, N=N passes (0 disables, max 10, default 1.0)")
	fmt.Fprintln(w, "  --no-curves           skip the background-darkening curves step (post_step6_curves.jpg)")
	fmt.Fprintln(w, "  --curves-black F      curves: shadow crush point (0-0.5, default 0.03)")
	fmt.Fprintln(w, "  --curves-gamma F      curves: midtone gamma (>1 darkens, <1 brightens, default 1.20)")
	fmt.Fprintln(w, "  --spikes              add diffraction spikes (default: on; --no-spikes to disable)")
	fmt.Fprintln(w, "  --no-spikes           disable diffraction spikes (overrides --spikes)")
	fmt.Fprintln(w, "  --spikes-percent F    percent of detected stars to spike (default 0.2)")
	fmt.Fprintln(w, "  --spikes-length F     spike arm half-length in px (default 100)")
	fmt.Fprintln(w, "  --spikes-intensity F  peak additive spike brightness (default 0.7)")
	fmt.Fprintln(w, "  --spikes-arms N       number of spike arms: 4=cross, 6=hexagonal (default 6)")
	fmt.Fprintln(w, "  --spikes-softness F   intensity falloff exponent: 0.1=flat, 0.5=sqrt, 1.0=linear (default 0.5)")
	fmt.Fprintln(w, "  --spikes-min-ratio F  minimum intensity for faintest spiked star (default 0.2)")
	fmt.Fprintln(w, "  --spikes-length-softness F  length falloff exponent: 1.0=proportional, 1.5=super-linear (default 1.0)")
	fmt.Fprintln(w, "  --spikes-length-min-ratio F minimum length for faintest spiked star (default 0.03)")
	fmt.Fprintln(w, "  --spikes-min-count N  minimum number of stars to spike, floors the percent-based selection (default 5)")
	fmt.Fprintln(w, "  --sigma-clip F        MAD sigma-clipping threshold for median/sigma stacks (default 3.0)")
	fmt.Fprintln(w, "  --sigma-clip-iters N  number of sigma-clipping iterations (default 2)")
	fmt.Fprintln(w, "  --tile-size N         tile side length for median/sigma reduction (default: auto)")
	fmt.Fprintln(w, "  --normalize       globally normalize each frame to the reference (requires registration)")
	fmt.Fprintln(w, "  --local-normalize per-tile normalization; implies --normalize (requires registration)")
	fmt.Fprintln(w, "  --ln-tile-size N  tile side length for --local-normalize (default 256)")
	fmt.Fprintln(w, "  --no-cache         disable star/transform cache (default: cache in <output>/_work/cache)")
	fmt.Fprintln(w, "  --cache-dir string cache directory (default: <output>/_work/cache)")
	fmt.Fprintln(w, "  --test-debayer P  test FITS debayering on one file: writes <base>.jpg and <base>_raw.jpg, then exits")
	fmt.Fprintln(w, "  --selftest        run internal self-tests and exit")
	fmt.Fprintln(w, "  --version         print version and exit")
	fmt.Fprintln(w, "  --help            print help")
}

// =========================================================================
// Discovery
// =========================================================================

func discoverFiles(root string, recursive bool, exclude []string) ([]string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	excludes := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		abs, err := filepath.Abs(e)
		if err != nil {
			return nil, err
		}
		excludes[abs] = true
	}
	var files []string
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if excludes[abs] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != absRoot && !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".cr2" || ext == ".cr3" || ext == ".fits" || ext == ".fit" {
			files = append(files, abs)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// =========================================================================
// Imaging
// =========================================================================

type frame struct {
	Width  int
	Height int
	Pix    []float32 // interleaved RGB in [0, 1], unless a pixel was marked invalid (-1)
}

func calibrate(f *frame, dark, flat, bias *frame) {
	if dark != nil {
		for i := range f.Pix {
			f.Pix[i] -= dark.Pix[i]
		}
	}
	if flat != nil {
		for i := range f.Pix {
			denom := flat.Pix[i]
			if bias != nil {
				denom -= bias.Pix[i]
			}
			if denom < 1e-5 {
				denom = 1e-5
			}
			f.Pix[i] /= denom
		}
	}
}

// findCR2Tool returns the first available CR2 decoder, or "".
func findCR2Tool() string {
	for _, name := range []string{"dcraw_emu", "dcraw"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

func decodeCR2Config(path string) (image.Config, error) {
	tool := findCR2Tool()
	if tool != "" {
		// dcraw -i (non-verbose) prints only "FNAME is a MAKE MODEL image."
		// to stderr, with no dimensions. -i -v prints additional metadata
		// (also to stderr) that includes "Output size: N x M". Combine
		// stdout+stderr in case a dcraw variant prints to stdout instead.
		cmd := exec.Command(tool, "-i", "-v", path)
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		_ = cmd.Run()
		text := outBuf.String() + errBuf.String()
		for _, pat := range []string{
			`Output size:\s*(\d+)\s*x\s*(\d+)`,
			`Image size:\s*(\d+)\s*x\s*(\d+)`,
			`Full size:\s*(\d+)\s*x\s*(\d+)`,
		} {
			re := regexp.MustCompile(pat)
			if m := re.FindStringSubmatch(text); len(m) == 3 {
				w, _ := strconv.Atoi(m[1])
				h, _ := strconv.Atoi(m[2])
				if w > 0 && h > 0 {
					return image.Config{Width: w, Height: h}, nil
				}
			}
		}
	}
	// Fallback: ImageMagick direct (works only with a raw delegate)
	cmd := exec.Command("identify", "-format", "%w %h", path)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return image.Config{}, fmt.Errorf("cannot read CR2 %s: dcraw ran but reported no dimensions, and ImageMagick also failed: %s", path, errBuf.String())
	}
	parts := strings.Fields(out.String())
	if len(parts) != 2 {
		return image.Config{}, fmt.Errorf("cannot read CR2 %s: unexpected identify output %q", path, out.String())
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	return image.Config{Width: w, Height: h}, nil
}

func decodeCR2File(path string, targetW, targetH int) (*frame, error) {
	tool := findCR2Tool()
	if tool != "" {
		dcraw := exec.Command(tool, "-c", "-T", "-4", path)
		var dcrawBuf, dcrawErr bytes.Buffer
		dcraw.Stdout = &dcrawBuf
		dcraw.Stderr = &dcrawErr
		if err := dcraw.Run(); err != nil {
			return nil, fmt.Errorf("%s failed on %s: %w\n  stderr: %s",
				tool, filepath.Base(path), err, strings.TrimSpace(dcrawErr.String()))
		}
		if dcrawBuf.Len() < 100 {
			return nil, fmt.Errorf("%s produced %d bytes for %s (expected TIFF data); stderr: %s",
				tool, dcrawBuf.Len(), filepath.Base(path), strings.TrimSpace(dcrawErr.String()))
		}
		convArgs := []string{"tiff:-"}
		if targetW > 0 && targetH > 0 {
			convArgs = append(convArgs, "-resize", fmt.Sprintf("%dx%d!", targetW, targetH))
		}
		convArgs = append(convArgs, "-depth", "16", "png:-")
		conv := exec.Command("convert", convArgs...)
		conv.Stdin = &dcrawBuf
		var convBuf, convErr bytes.Buffer
		conv.Stdout = &convBuf
		conv.Stderr = &convErr
		if err := conv.Run(); err != nil {
			return nil, fmt.Errorf("CR2->PNG convert failed on %s: %w\n  stderr: %s",
				filepath.Base(path), err, strings.TrimSpace(convErr.String()))
		}
		img, _, err := image.Decode(&convBuf)
		if err != nil {
			return nil, fmt.Errorf("decode CR2 PNG (%s): %w", filepath.Base(path), err)
		}
		return frameFromImage(img)
	}
	// Fallback: direct ImageMagick (requires a raw delegate)
	args := []string{path}
	if targetW > 0 && targetH > 0 {
		args = append(args, "-resize", fmt.Sprintf("%dx%d!", targetW, targetH))
	}
	args = append(args, "-depth", "16", "png:-")
	cmd := exec.Command("convert", args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("cannot decode CR2 %s: no dcraw/dcraw_emu in PATH and ImageMagick has no raw delegate: %s",
			filepath.Base(path), strings.TrimSpace(errBuf.String()))
	}
	img, _, err := image.Decode(&out)
	if err != nil {
		return nil, err
	}
	return frameFromImage(img)
}

func decodeImageConfig(path string) (image.Config, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".cr2" || ext == ".cr3" {
		return decodeCR2Config(path)
	}
	if ext == ".fits" || ext == ".fit" {
		w, h, _, _, _, _, _, _, _, err := parseFITSHeader(path)
		if err == nil && w > 0 && h > 0 {
			return image.Config{Width: w, Height: h}, nil
		}
		cmd := exec.Command("identify", "-format", "%w %h", path)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err = cmd.Run(); err != nil {
			return image.Config{}, fmt.Errorf("identify %s failed: %w", ext, err)
		}
		parts := strings.Fields(out.String())
		if len(parts) != 2 {
			return image.Config{}, errors.New("invalid dimensions")
		}
		w, _ = strconv.Atoi(parts[0])
		h, _ = strconv.Atoi(parts[1])
		return image.Config{Width: w, Height: h}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	return cfg, err
}

func decodeImageFile(path string, targetW, targetH int) (*frame, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".cr2" || ext == ".cr3" {
		return decodeCR2File(path, targetW, targetH)
	}
	if ext == ".fits" || ext == ".fit" {
		frm, err := decodeFITSNative(path, targetW, targetH)
		if err == nil {
			return frm, nil
		}
		args := []string{path}
		if targetW > 0 && targetH > 0 {
			args = append(args, "-resize", fmt.Sprintf("%dx%d!", targetW, targetH))
		}
		args = append(args, "-depth", "16", "png:-")
		cmd := exec.Command("convert", args...)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err = cmd.Run(); err != nil {
			return nil, fmt.Errorf("decode %s failed: %w", ext, err)
		}
		img, _, err := image.Decode(&out)
		if err != nil {
			return nil, err
		}
		return frameFromImage(img)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	frm, err := frameFromImage(img)
	if err != nil {
		return nil, err
	}
	if targetW > 0 && targetH > 0 && (frm.Width != targetW || frm.Height != targetH) {
		return resizeFrame(frm, targetW, targetH), nil
	}
	return frm, nil
}

func resizeFrame(f *frame, w, h int) *frame {
	if f.Width == w && f.Height == h {
		return f
	}
	out := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	scaleX := float64(f.Width) / float64(w)
	scaleY := float64(f.Height) / float64(h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcX := float64(x) * scaleX
			srcY := float64(y) * scaleY
			x0, y0 := int(srcX), int(srcY)
			x1, y1 := x0+1, y0+1
			if x1 >= f.Width {
				x1 = f.Width - 1
			}
			if y1 >= f.Height {
				y1 = f.Height - 1
			}
			fx, fy := float32(srcX-float64(x0)), float32(srcY-float64(y0))
			for c := 0; c < 3; c++ {
				p00 := f.Pix[(y0*f.Width+x0)*3+c]
				p10 := f.Pix[(y0*f.Width+x1)*3+c]
				p01 := f.Pix[(y1*f.Width+x0)*3+c]
				p11 := f.Pix[(y1*f.Width+x1)*3+c]
				out.Pix[(y*w+x)*3+c] = (1-fy)*((1-fx)*p00+fx*p10) + fy*((1-fx)*p01+fx*p11)
			}
		}
	}
	return out
}

func frameFromImage(img image.Image) (*frame, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid image dimensions %dx%d", w, h)
	}
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	switch src := img.(type) {
	case *image.YCbCr:
		const inv = 1.0 / 255.0
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				yi := src.YOffset(b.Min.X+x, b.Min.Y+y)
				ci := src.COffset(b.Min.X+x, b.Min.Y+y)
				r, g, bl := color.YCbCrToRGB(src.Y[yi], src.Cb[ci], src.Cr[ci])
				i := (y*w + x) * 3
				f.Pix[i+0] = float32(r) * inv
				f.Pix[i+1] = float32(g) * inv
				f.Pix[i+2] = float32(bl) * inv
			}
		}
	default:
		const inv = 1.0 / 65535.0
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
				i := (y*w + x) * 3
				f.Pix[i+0] = float32(r) * inv
				f.Pix[i+1] = float32(g) * inv
				f.Pix[i+2] = float32(bl) * inv
			}
		}
	}
	return f, nil
}

// grayscale returns a luminance image in [0, 1].
func grayscale(f *frame) []float32 {
	n := f.Width * f.Height
	g := make([]float32, n)
	for i := 0; i < n; i++ {
		g[i] = 0.299*f.Pix[i*3] + 0.587*f.Pix[i*3+1] + 0.114*f.Pix[i*3+2]
	}
	return g
}

// =========================================================================
// Star detection
// =========================================================================

type star struct {
	X, Y float64 // sub-pixel centroid
	Flux float64 // background-subtracted integrated intensity
	Size int     // connected component pixel count
}

// estimateBackgroundNoise uses a 256-bin histogram over a subsample to
// return the median (background level) and an IQR-based Gaussian sigma.
func estimateBackgroundNoise(gray []float32) (bg, noise float64) {
	const numBins = 256
	var hist [numBins]int64
	total := int64(0)
	step := 4
	for i := 0; i < len(gray); i += step {
		v := gray[i]
		if v < 0 {
			v = 0
		} else if v >= 1 {
			v = 0.999999
		}
		hist[int(v*numBins)]++
		total++
	}
	if total == 0 {
		return 0, 1e-6
	}
	p25, p50, p75 := int64(0), int64(0), int64(0)
	want := []int64{total * 25 / 100, total * 50 / 100, total * 75 / 100}
	cum := int64(0)
	idx := 0
	for i := 0; i < numBins && idx < 3; i++ {
		cum += hist[i]
		for idx < 3 && cum >= want[idx] {
			switch idx {
			case 0:
				p25 = int64(i)
			case 1:
				p50 = int64(i)
			case 2:
				p75 = int64(i)
			}
			idx++
		}
	}
	bg = float64(p50) / numBins
	iqr := float64(p75-p25) / numBins
	noise = iqr / 1.349
	if noise < 1e-6 {
		noise = 1e-6
	}
	return bg, noise
}

// detectStars returns up to maxStars stars sorted by decreasing flux.
//
// Tries the standard 5-sigma threshold first. If that yields very few
// stars (e.g. faint, low-contrast frames, or frames dominated by nebulosity),
// retries with progressively lower thresholds so the caller still gets a
// workable star list. This is a simple form of multi-scale matching: at
// each level the threshold is relaxed until at least ~20 stars are found.
func detectStars(f *frame, maxStars int) []star {
	stars := detectStarsSigma(f, maxStars, 5.0)
	if len(stars) >= 20 {
		return stars
	}
	for _, sig := range []float64{4.0, 3.5, 3.0} {
		s := detectStarsSigma(f, maxStars, sig)
		if len(s) > len(stars) {
			stars = s
		}
		if len(stars) >= 20 {
			break
		}
	}
	return stars
}

// boxBlurLuminance returns a box-blurred copy of gray using the given
// radius. It is a separable two-pass filter (horizontal then vertical), so
// cost is O(W*H) regardless of radius, and edges are handled by clamping
// the window to [0, W-1] / [0, H-1].
func boxBlurLuminance(gray []float32, w, h, radius int) []float32 {
	if radius < 1 {
		radius = 1
	}
	tmp := make([]float32, len(gray))
	prefixX := make([]float64, w+1)
	for y := 0; y < h; y++ {
		rowOff := y * w
		prefixX[0] = 0
		for x := 0; x < w; x++ {
			prefixX[x+1] = prefixX[x] + float64(gray[rowOff+x])
		}
		for x := 0; x < w; x++ {
			x0 := x - radius
			if x0 < 0 {
				x0 = 0
			}
			x1 := x + radius + 1
			if x1 > w {
				x1 = w
			}
			tmp[rowOff+x] = float32((prefixX[x1] - prefixX[x0]) / float64(x1-x0))
		}
	}
	out := make([]float32, len(gray))
	prefixY := make([]float64, h+1)
	for x := 0; x < w; x++ {
		prefixY[0] = 0
		for y := 0; y < h; y++ {
			prefixY[y+1] = prefixY[y] + float64(tmp[y*w+x])
		}
		for y := 0; y < h; y++ {
			y0 := y - radius
			if y0 < 0 {
				y0 = 0
			}
			y1 := y + radius + 1
			if y1 > h {
				y1 = h
			}
			out[y*w+x] = float32((prefixY[y1] - prefixY[y0]) / float64(y1-y0))
		}
	}
	return out
}

// estimateResidualNoise returns the median absolute deviation (times
// 1.4826) of a zero-centred residual, which is the standard robust
// estimate of the background noise sigma.
func estimateResidualNoise(residual []float32) float64 {
	const step = 4
	n := (len(residual) + step - 1) / step
	sample := make([]float32, 0, n)
	for i := 0; i < len(residual); i += step {
		v := residual[i]
		if v < 0 {
			v = -v
		}
		sample = append(sample, v)
	}
	if len(sample) == 0 {
		return 1e-6
	}
	sort.Slice(sample, func(i, j int) bool { return sample[i] < sample[j] })
	mad := float64(sample[len(sample)/2])
	sigma := 1.4826 * mad
	if sigma < 1e-6 {
		sigma = 1e-6
	}
	return sigma
}

// estimateHighPassRadius returns a star-detection high-pass radius sized
// to the image. The old fixed radius (12 px) was fine for ~1000-3000 px
// images where stars are typically 2-6 px across, but is too small for
// 4000+ px images (where stars can be 5-12 px) and too large for small
// previews. Scaling with the shorter edge keeps the radius comparable to
// the typical star size across image scales, so the same threshold works
// from a 500 px test image up to a 6000 px full-frame stack.
func estimateHighPassRadius(w, h int) int {
	m := w
	if h < m {
		m = h
	}
	r := m / 200
	if r < 8 {
		r = 8
	}
	if r > 24 {
		r = 24
	}
	return r
}

func detectStarsSigma(f *frame, maxStars int, sigma float64) []star {
	w, h := f.Width, f.Height
	gray := grayscale(f)
	// High-pass filter: subtract a large-scale box blur so the local
	// background (dark sky, nebula, galaxy disk, vignetting) is removed
	// before thresholding. Without this, a single global threshold either
	// misses stars on bright backgrounds (they do not stand out) or lets
	// the whole nebula merge into one giant rejected component.
	//
	// Radius must exceed the largest star we want to catch (typically
	// <= 6 px) but stay below the scale of any real nebula structure.
	// estimateHighPassRadius scales with the image so the same threshold
	// works on both small previews and full-frame stacks.
	hpRadius := estimateHighPassRadius(w, h)
	blurred := boxBlurLuminance(gray, w, h, hpRadius)
	residual := make([]float32, len(gray))
	for i := range gray {
		residual[i] = gray[i] - blurred[i]
	}
	noise := estimateResidualNoise(residual)
	threshold := float32(sigma * noise)
	if threshold < 1.0/512.0 {
		threshold = 1.0 / 512.0
	}

	visited := make([]bool, w*h)
	type comp struct {
		pixels []int
	}
	var comps []comp

	// Iterative flood fill for 8-connectivity
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			if visited[i] || residual[i] <= threshold {
				continue
			}
			// BFS
			stack := []int{i}
			visited[i] = true
			var pixels []int
			for len(stack) > 0 {
				p := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				pixels = append(pixels, p)
				py, px := p/w, p%w
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if dx == 0 && dy == 0 {
							continue
						}
						nx, ny := px+dx, py+dy
						if nx < 0 || ny < 0 || nx >= w || ny >= h {
							continue
						}
						ni := ny*w + nx
						if visited[ni] || residual[ni] <= threshold {
							continue
						}
						visited[ni] = true
						stack = append(stack, ni)
					}
				}
			}
			if len(pixels) >= 1 && len(pixels) <= 200 {
				comps = append(comps, comp{pixels: pixels})
			}
		}
	}

	stars := make([]star, 0, len(comps))
	const margin = 3
	for _, c := range comps {
		var sumW, sumX, sumY, sumFlux float64
		minX, minY, maxX, maxY := w, h, -1, -1
		for _, p := range c.pixels {
			py, px := p/w, p%w
			val := float64(residual[p])
			if val < 0 {
				val = 0
			}
			sumW += val
			sumX += float64(px) * val
			sumY += float64(py) * val
			sumFlux += val
			if px < minX {
				minX = px
			}
			if px > maxX {
				maxX = px
			}
			if py < minY {
				minY = py
			}
			if py > maxY {
				maxY = py
			}
		}
		if sumW == 0 {
			continue
		}
		cx := sumX / sumW
		cy := sumY / sumW
		if cx < margin || cy < margin || cx > float64(w-margin) || cy > float64(h-margin) {
			continue
		}
		// Shape filter: reject very elongated blobs
		bw, bh := maxX-minX+1, maxY-minY+1
		aspect := float64(bw) / float64(bh)
		if aspect < 0.33 || aspect > 3.0 {
			continue
		}
		stars = append(stars, star{X: cx, Y: cy, Flux: sumFlux, Size: len(c.pixels)})
	}
	sort.Slice(stars, func(i, j int) bool { return stars[i].Flux > stars[j].Flux })
	if len(stars) > maxStars {
		stars = stars[:maxStars]
	}
	return stars
}

// =========================================================================
// Similarity transform
// =========================================================================

type point2D struct {
	X, Y float64
}

// similarity represents p' = s·R·p + t, encoded as:
//
//	p'.X = SC·p.X - SS·p.Y + Tx
//	p'.Y = SS·p.X + SC·p.Y + Ty
//
// with s = sqrt(SC² + SS²), θ = atan2(SS, SC).
type similarity struct {
	SC, SS, Tx, Ty float64
}

var identitySimilarity = similarity{SC: 1, SS: 0, Tx: 0, Ty: 0}

func (s similarity) apply(p point2D) point2D {
	return point2D{
		X: s.SC*p.X - s.SS*p.Y + s.Tx,
		Y: s.SS*p.X + s.SC*p.Y + s.Ty,
	}
}

func (s similarity) inverse() similarity {
	det := s.SC*s.SC + s.SS*s.SS
	if det < 1e-12 {
		return identitySimilarity
	}
	invSC := s.SC / det
	invSS := -s.SS / det
	return similarity{
		SC: invSC,
		SS: invSS,
		Tx: -(invSC*s.Tx - invSS*s.Ty),
		Ty: -(invSS*s.Tx + invSC*s.Ty),
	}
}

func (s similarity) scale() float64 {
	return math.Sqrt(s.SC*s.SC + s.SS*s.SS)
}

func (s similarity) rotation() float64 {
	return math.Atan2(s.SS, s.SC)
}

// estimateSimilarity returns the least-squares similarity mapping src→dst.
func estimateSimilarity(src, dst []point2D) similarity {
	n := len(src)
	if n < 2 || n != len(dst) {
		return identitySimilarity
	}
	var scX, scY, dcX, dcY float64
	for i := 0; i < n; i++ {
		scX += src[i].X
		scY += src[i].Y
		dcX += dst[i].X
		dcY += dst[i].Y
	}
	scX /= float64(n)
	scY /= float64(n)
	dcX /= float64(n)
	dcY /= float64(n)
	var num1, num2, den float64
	for i := 0; i < n; i++ {
		sx := src[i].X - scX
		sy := src[i].Y - scY
		dx := dst[i].X - dcX
		dy := dst[i].Y - dcY
		num1 += sx*dx + sy*dy
		num2 += sx*dy - sy*dx
		den += sx*sx + sy*sy
	}
	if den < 1e-12 {
		return identitySimilarity
	}
	sc := num1 / den
	ss := num2 / den
	return similarity{
		SC: sc,
		SS: ss,
		Tx: dcX - (sc*scX - ss*scY),
		Ty: dcY - (ss*scX + sc*scY),
	}
}

// affine is a full 6-DOF 2D affine transform:
//
//	x' = A*x + B*y + C
//	y' = D*x + E*y + F
//
// It subsumes similarity (SC, SS, Tx, Ty) when the matrix is a scaled
// rotation, but also handles anisotropic scale and shear. That makes it
// a better fit than a pure similarity for wide-field optics that show
// small astigmatism or field rotation across the frame. Full field
// distortion (SIP / plate solving) would require a higher-order model
// and is out of scope for this single-file stacker.
type affine struct {
	A, B, C, D, E, F float64
}

var identityAffine = affine{A: 1, B: 0, C: 0, D: 0, E: 1, F: 0}

func (a affine) apply(p point2D) point2D {
	return point2D{
		X: a.A*p.X + a.B*p.Y + a.C,
		Y: a.D*p.X + a.E*p.Y + a.F,
	}
}

// inverse returns the affine transform that undoes a, or identityAffine
// if a is singular.
func (a affine) inverse() affine {
	det := a.A*a.E - a.B*a.D
	if math.Abs(det) < 1e-12 {
		return identityAffine
	}
	invDet := 1.0 / det
	return affine{
		A: a.E * invDet,
		B: -a.B * invDet,
		C: (a.B*a.F - a.E*a.C) * invDet,
		D: -a.D * invDet,
		E: a.A * invDet,
		F: (a.D*a.C - a.A*a.F) * invDet,
	}
}

// scale returns an approximate uniform scale (sqrt of |det|). For a
// pure similarity this matches similarity.scale(); for a general
// affine it is the geometric mean of the two singular values.
func (a affine) scale() float64 {
	det := a.A*a.E - a.B*a.D
	if det < 0 {
		det = -det
	}
	return math.Sqrt(det)
}

// rotation returns an approximate rotation angle in radians. For a pure
// rotation this is exactly the rotation; for a general affine it is a
// best-effort estimate suitable for the quality diagnostic.
func (a affine) rotation() float64 {
	return math.Atan2(a.D-a.B, a.A+a.E)
}

// estimateAffine returns the least-squares full affine transform mapping
// src -> dst via the 3x3 normal equations. Returns ok=false if the system
// is singular (fewer than 3 non-collinear points) or if the source points
// have zero variance.
//
// The system is normalised internally (centroid subtraction, RMS scaling)
// so the same routine works whether the caller passes image pixels or
// sub-pixel star centroids.
func estimateAffine(src, dst []point2D) (affine, bool) {
	n := len(src)
	if n < 3 || n != len(dst) {
		return affine{}, false
	}
	var scx, scy float64
	for i := 0; i < n; i++ {
		scx += src[i].X
		scy += src[i].Y
	}
	scx /= float64(n)
	scy /= float64(n)
	var srms float64
	for i := 0; i < n; i++ {
		dx := src[i].X - scx
		dy := src[i].Y - scy
		srms += dx*dx + dy*dy
	}
	srms = math.Sqrt(srms / float64(n))
	if srms < 1e-9 {
		return affine{}, false
	}
	var M [3][3]float64
	var bx, by [3]float64
	for i := 0; i < n; i++ {
		xn := (src[i].X - scx) / srms
		yn := (src[i].Y - scy) / srms
		M[0][0] += 1
		M[0][1] += xn
		M[0][2] += yn
		M[1][0] += xn
		M[1][1] += xn * xn
		M[1][2] += xn * yn
		M[2][0] += yn
		M[2][1] += xn * yn
		M[2][2] += yn * yn
		bx[0] += dst[i].X
		bx[1] += dst[i].X * xn
		bx[2] += dst[i].X * yn
		by[0] += dst[i].Y
		by[1] += dst[i].Y * xn
		by[2] += dst[i].Y * yn
	}
	det := M[0][0]*(M[1][1]*M[2][2]-M[1][2]*M[2][1]) -
		M[0][1]*(M[1][0]*M[2][2]-M[1][2]*M[2][0]) +
		M[0][2]*(M[1][0]*M[2][1]-M[1][1]*M[2][0])
	if math.Abs(det) < 1e-12 {
		return affine{}, false
	}
	invDet := 1.0 / det
	var inv [3][3]float64
	inv[0][0] = (M[1][1]*M[2][2] - M[1][2]*M[2][1]) * invDet
	inv[0][1] = -(M[0][1]*M[2][2] - M[0][2]*M[2][1]) * invDet
	inv[0][2] = (M[0][1]*M[1][2] - M[0][2]*M[1][1]) * invDet
	inv[1][0] = -(M[1][0]*M[2][2] - M[1][2]*M[2][0]) * invDet
	inv[1][1] = (M[0][0]*M[2][2] - M[0][2]*M[2][0]) * invDet
	inv[1][2] = -(M[0][0]*M[1][2] - M[0][2]*M[1][0]) * invDet
	inv[2][0] = (M[1][0]*M[2][1] - M[1][1]*M[2][0]) * invDet
	inv[2][1] = -(M[0][0]*M[2][1] - M[0][1]*M[2][0]) * invDet
	inv[2][2] = (M[0][0]*M[1][1] - M[0][1]*M[1][0]) * invDet
	var cx, cy [3]float64
	for j := 0; j < 3; j++ {
		for k := 0; k < 3; k++ {
			cx[j] += inv[j][k] * bx[k]
			cy[j] += inv[j][k] * by[k]
		}
	}
	return affine{
		A: cx[1] / srms,
		B: cx[2] / srms,
		C: cx[0] - cx[1]*scx/srms - cx[2]*scy/srms,
		D: cy[1] / srms,
		E: cy[2] / srms,
		F: cy[0] - cy[1]*scx/srms - cy[2]*scy/srms,
	}, true
}

// =========================================================================
// Triangle-based star matching
// =========================================================================

type triangle struct {
	A, B, C int     // canonical vertex indices: A opposite shortest side, C opposite longest
	R1, R2  float64 // side ratios (short/long, mid/long)
}

func dist2(a, b star) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return dx*dx + dy*dy
}

// buildTriangles forms canonical triangles from the top-N brightest stars.
func buildTriangles(stars []star) []triangle {
	n := len(stars)
	if n > 40 {
		n = 40
	}
	if n < 3 {
		return nil
	}
	var out []triangle
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			dij2 := dist2(stars[i], stars[j])
			if dij2 < 25 { // reject very short sides
				continue
			}
			for k := j + 1; k < n; k++ {
				dik2 := dist2(stars[i], stars[k])
				djk2 := dist2(stars[j], stars[k])
				if dik2 < 25 || djk2 < 25 {
					continue
				}
				// sides[] = (length, opposite-vertex-index)
				sides := [3][2]float64{
					{math.Sqrt(djk2), float64(i)}, // opposite i
					{math.Sqrt(dik2), float64(j)}, // opposite j
					{math.Sqrt(dij2), float64(k)}, // opposite k
				}
				// Sort ascending by length
				if sides[1][0] < sides[0][0] {
					sides[0], sides[1] = sides[1], sides[0]
				}
				if sides[2][0] < sides[1][0] {
					sides[1], sides[2] = sides[2], sides[1]
				}
				if sides[1][0] < sides[0][0] {
					sides[0], sides[1] = sides[1], sides[0]
				}
				// Reject near-isosceles top two (ambiguous ordering)
				if sides[2][0]-sides[1][0] < 0.05*sides[2][0] {
					continue
				}
				// Reject elongated triangles
				if sides[0][0] < 0.15*sides[2][0] {
					continue
				}
				out = append(out, triangle{
					A:  int(sides[0][1]),
					B:  int(sides[1][1]),
					C:  int(sides[2][1]),
					R1: sides[0][0] / sides[2][0],
					R2: sides[1][0] / sides[2][0],
				})
			}
		}
	}
	return out
}

type triangleMatch struct {
	Ref, Tgt [3]int // star indices in ref / target frame
}

// chirality returns the sign of (B-A) × (C-A).
func chirality(stars []star, a, b, c int) float64 {
	ax, ay := stars[a].X, stars[a].Y
	bx, by := stars[b].X, stars[b].Y
	cx, cy := stars[c].X, stars[c].Y
	return (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
}

func matchTriangles(refStars, tgtStars []star) []triangleMatch {
	refTris := buildTriangles(refStars)
	tgtTris := buildTriangles(tgtStars)
	if len(refTris) == 0 || len(tgtTris) == 0 {
		return nil
	}
	// Bucket reference triangles by descriptor into a 0.01 grid.
	const grid = 100
	bucket := make(map[[2]int][]int, len(refTris))
	for i, rt := range refTris {
		k := [2]int{int(rt.R1 * grid), int(rt.R2 * grid)}
		bucket[k] = append(bucket[k], i)
	}
	const descTol = 0.02
	var out []triangleMatch
	for _, tt := range tgtTris {
		key := [2]int{int(tt.R1 * grid), int(tt.R2 * grid)}
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				for _, ri := range bucket[[2]int{key[0] + dx, key[1] + dy}] {
					rt := refTris[ri]
					d1 := rt.R1 - tt.R1
					d2 := rt.R2 - tt.R2
					if d1*d1+d2*d2 > descTol*descTol {
						continue
					}
					// Chirality must agree for a proper similarity.
					refChir := chirality(refStars, rt.A, rt.B, rt.C)
					tgtChir := chirality(tgtStars, tt.A, tt.B, tt.C)
					if (refChir > 0) != (tgtChir > 0) {
						continue
					}
					out = append(out, triangleMatch{
						Ref: [3]int{rt.A, rt.B, rt.C},
						Tgt: [3]int{tt.A, tt.B, tt.C},
					})
				}
			}
		}
	}
	return out
}

// countInliers returns the number of target stars whose transformed position
// lies within tol of a reference star. Uses a simple linear scan; fine for
// ~100 stars.
func countInliers(sim similarity, tgtStars, refStars []star, tol float64) int {
	tol2 := tol * tol
	n := 0
	for _, ts := range tgtStars {
		p := sim.apply(point2D{ts.X, ts.Y})
		for _, rs := range refStars {
			dx := p.X - rs.X
			dy := p.Y - rs.Y
			if dx*dx+dy*dy <= tol2 {
				n++
				break
			}
		}
	}
	return n
}

// collectInliers returns matched (src, dst) point pairs under sim within tol.
func collectInliers(sim similarity, tgtStars, refStars []star, tol float64) ([]point2D, []point2D) {
	tol2 := tol * tol
	var src, dst []point2D
	used := make([]bool, len(refStars))
	for _, ts := range tgtStars {
		p := sim.apply(point2D{ts.X, ts.Y})
		bestJ, bestD := -1, tol2
		for j, rs := range refStars {
			if used[j] {
				continue
			}
			dx := p.X - rs.X
			dy := p.Y - rs.Y
			d := dx*dx + dy*dy
			if d <= bestD {
				bestJ = j
				bestD = d
			}
		}
		if bestJ >= 0 {
			used[bestJ] = true
			src = append(src, point2D{ts.X, ts.Y})
			dst = append(dst, point2D{refStars[bestJ].X, refStars[bestJ].Y})
		}
	}
	return src, dst
}

// matchStarsToReference returns the similarity that maps target stars onto
// reference stars, plus the inlier count. Returns ok=false if the match is
// not confident enough.
func matchStarsToReference(refStars, tgtStars []star) (similarity, int, bool) {
	const tol = 3.0
	matches := matchTriangles(refStars, tgtStars)
	if len(matches) == 0 {
		return identitySimilarity, 0, false
	}
	bestInliers := 0
	bestSim := identitySimilarity
	for _, m := range matches {
		src := []point2D{
			{tgtStars[m.Tgt[0]].X, tgtStars[m.Tgt[0]].Y},
			{tgtStars[m.Tgt[1]].X, tgtStars[m.Tgt[1]].Y},
			{tgtStars[m.Tgt[2]].X, tgtStars[m.Tgt[2]].Y},
		}
		dst := []point2D{
			{refStars[m.Ref[0]].X, refStars[m.Ref[0]].Y},
			{refStars[m.Ref[1]].X, refStars[m.Ref[1]].Y},
			{refStars[m.Ref[2]].X, refStars[m.Ref[2]].Y},
		}
		sim := estimateSimilarity(src, dst)
		if sim.scale() < 0.5 || sim.scale() > 2.0 {
			continue
		}
		ni := countInliers(sim, tgtStars, refStars, tol)
		if ni > bestInliers {
			bestInliers = ni
			bestSim = sim
		}
	}
	minCount := len(tgtStars)
	if len(refStars) < minCount {
		minCount = len(refStars)
	}
	if minCount == 0 {
		return identitySimilarity, bestInliers, false
	}
	ratio := float64(bestInliers) / float64(minCount)
	if bestInliers < 8 || ratio < 0.20 {
		return identitySimilarity, bestInliers, false
	}
	// Refine with all inliers
	src, dst := collectInliers(bestSim, tgtStars, refStars, tol)
	if len(src) >= 2 {
		bestSim = estimateSimilarity(src, dst)
		bestInliers = len(src)
	}
	return bestSim, bestInliers, true
}

// =========================================================================
// Resampling and cropping
// =========================================================================

// sampleBilinear samples f at sub-pixel (x, y). Returns ok=false if (x, y)
// lies outside the source bounds.
func sampleBilinear(f *frame, x, y float64) (r, g, b float32, ok bool) {
	if x < 0 || x > float64(f.Width-1) || y < 0 || y > float64(f.Height-1) {
		return 0, 0, 0, false
	}
	x0 := int(x)
	y0 := int(y)
	fx := float32(x - float64(x0))
	fy := float32(y - float64(y0))
	x1 := x0 + 1
	if x1 >= f.Width {
		x1 = f.Width - 1
	}
	y1 := y0 + 1
	if y1 >= f.Height {
		y1 = f.Height - 1
	}
	i00 := (y0*f.Width + x0) * 3
	i10 := (y0*f.Width + x1) * 3
	i01 := (y1*f.Width + x0) * 3
	i11 := (y1*f.Width + x1) * 3
	w00 := (1 - fx) * (1 - fy)
	w10 := fx * (1 - fy)
	w01 := (1 - fx) * fy
	w11 := fx * fy
	r = w00*f.Pix[i00] + w10*f.Pix[i10] + w01*f.Pix[i01] + w11*f.Pix[i11]
	g = w00*f.Pix[i00+1] + w10*f.Pix[i10+1] + w01*f.Pix[i01+1] + w11*f.Pix[i11+1]
	b = w00*f.Pix[i00+2] + w10*f.Pix[i10+2] + w01*f.Pix[i01+2] + w11*f.Pix[i11+2]
	return r, g, b, true
}

// resampleToReference produces a refW×refH frame in the reference
// coordinate system. Pixels with no source are marked -1. sim maps
// source -> reference, so its inverse maps reference -> source.
func resampleToReference(f *frame, sim similarity, refW, refH, workers int) *frame {
	inv := sim.inverse()
	return resampleToReferenceFn(f, func(x, y float64) (float64, float64) {
		p := inv.apply(point2D{x, y})
		return p.X, p.Y
	}, refW, refH, workers)
}

// resampleToReferenceAffine is the affine-transform variant of
// resampleToReference. Use it when registration was refined to a full
// affine model; otherwise resampleToReference with the similarity model
// is the right call.
func resampleToReferenceAffine(f *frame, aff affine, refW, refH, workers int) *frame {
	inv := aff.inverse()
	return resampleToReferenceFn(f, func(x, y float64) (float64, float64) {
		p := inv.apply(point2D{x, y})
		return p.X, p.Y
	}, refW, refH, workers)
}

// resampleToReferenceFn is the shared body of the two public resamplers.
// sampleRef maps a reference coordinate (x, y) to the source coordinate
// that should be sampled there.
func resampleToReferenceFn(f *frame, sampleRef func(x, y float64) (float64, float64), refW, refH, workers int) *frame {
	out := &frame{Width: refW, Height: refH, Pix: make([]float32, refW*refH*3)}
	rowBand := func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			fy := float64(y)
			rowBase := y * refW * 3
			for x := 0; x < refW; x++ {
				fx := float64(x)
				sx, sy := sampleRef(fx, fy)
				i := rowBase + x*3
				r, g, b, ok := sampleBilinear(f, sx, sy)
				if ok {
					out.Pix[i] = r
					out.Pix[i+1] = g
					out.Pix[i+2] = b
				} else {
					out.Pix[i] = -1
					out.Pix[i+1] = -1
					out.Pix[i+2] = -1
				}
			}
		}
	}
	if workers <= 1 || refH < workers {
		rowBand(0, refH)
		return out
	}
	var wg sync.WaitGroup
	rowsPerWorker := (refH + workers - 1) / workers
	for w := 0; w < workers; w++ {
		y0 := w * rowsPerWorker
		if y0 >= refH {
			break
		}
		y1 := y0 + rowsPerWorker
		if y1 > refH {
			y1 = refH
		}
		wg.Add(1)
		go func(y0, y1 int) {
			defer wg.Done()
			rowBand(y0, y1)
		}(y0, y1)
	}
	wg.Wait()
	return out
}

// findSafeCrop returns the largest axis-aligned rectangle in which every
// pixel has validCount == requiredCount. Uses maximal-rectangle-in-histogram.
func findSafeCrop(validCount []int32, w, h int, requiredCount int32) image.Rectangle {
	heights := make([]int, w)
	bestArea := 0
	best := image.Rect(0, 0, 0, 0)
	for y := 0; y < h; y++ {
		rowBase := y * w
		for x := 0; x < w; x++ {
			if validCount[rowBase+x] == requiredCount {
				heights[x]++
			} else {
				heights[x] = 0
			}
		}
		stack := []int{-1}
		for x := 0; x <= w; x++ {
			hx := 0
			if x < w {
				hx = heights[x]
			}
			for len(stack) > 1 && heights[stack[len(stack)-1]] > hx {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				left := stack[len(stack)-1] + 1
				hgt := heights[top]
				area := hgt * (x - left)
				if area > bestArea {
					bestArea = area
					best = image.Rect(left, y-hgt+1, x, y+1)
				}
			}
			stack = append(stack, x)
		}
	}
	if bestArea == 0 {
		return image.Rect(0, 0, w, h)
	}
	return best
}

// =========================================================================
// Accumulator
// =========================================================================

type accumulator struct {
	Width, Height int
	Pix           []float32
	Count         int
}

func newAccumulator(w, h int) *accumulator {
	return &accumulator{Width: w, Height: h, Pix: make([]float32, w*h*3)}
}

func (a *accumulator) addFrame(f *frame, workers int) error {
	if f.Width != a.Width || f.Height != a.Height {
		return fmt.Errorf("frame dimensions %dx%d do not match accumulator %dx%d",
			f.Width, f.Height, a.Width, a.Height)
	}
	if len(f.Pix) != len(a.Pix) {
		return fmt.Errorf("pixel buffer length mismatch")
	}
	addRows := func(start, end int) {
		for i := start; i < end; i++ {
			a.Pix[i] += f.Pix[i]
		}
	}
	if workers <= 1 || a.Height < workers {
		addRows(0, len(a.Pix))
	} else {
		var wg sync.WaitGroup
		rowsPerWorker := (a.Height + workers - 1) / workers
		for w := 0; w < workers; w++ {
			y0 := w * rowsPerWorker
			if y0 >= a.Height {
				break
			}
			y1 := y0 + rowsPerWorker
			if y1 > a.Height {
				y1 = a.Height
			}
			wg.Add(1)
			go func(y0, y1 int) {
				defer wg.Done()
				addRows(y0*a.Width*3, y1*a.Width*3)
			}(y0, y1)
		}
		wg.Wait()
	}
	a.Count++
	return nil
}

// addFrameMasked skips pixels marked -1 and increments validCount per pixel.
func (a *accumulator) addFrameMasked(f *frame, workers int, validCount []int32) error {
	if f.Width != a.Width || f.Height != a.Height {
		return fmt.Errorf("frame dimensions mismatch")
	}
	addRows := func(startPix, endPix int) {
		for p := startPix; p < endPix; p++ {
			r := f.Pix[p*3]
			g := f.Pix[p*3+1]
			b := f.Pix[p*3+2]
			if r >= 0 && g >= 0 && b >= 0 {
				a.Pix[p*3] += r
				a.Pix[p*3+1] += g
				a.Pix[p*3+2] += b
				validCount[p]++
			}
		}
	}
	nPix := a.Width * a.Height
	if workers <= 1 || a.Height < workers {
		addRows(0, nPix)
	} else {
		var wg sync.WaitGroup
		rowsPerWorker := (a.Height + workers - 1) / workers
		for w := 0; w < workers; w++ {
			y0 := w * rowsPerWorker
			if y0 >= a.Height {
				break
			}
			y1 := y0 + rowsPerWorker
			if y1 > a.Height {
				y1 = a.Height
			}
			wg.Add(1)
			go func(y0, y1 int) {
				defer wg.Done()
				addRows(y0*a.Width, y1*a.Width)
			}(y0, y1)
		}
		wg.Wait()
	}
	a.Count++
	return nil
}

func (a *accumulator) divide() {
	if a.Count == 0 {
		return
	}
	inv := 1.0 / float32(a.Count)
	for i := range a.Pix {
		a.Pix[i] *= inv
	}
}

func (a *accumulator) divideByCount(counts []int32) {
	for i := 0; i < len(a.Pix); i++ {
		c := counts[i/3]
		if c > 0 {
			a.Pix[i] /= float32(c)
		} else {
			a.Pix[i] = 0
		}
	}
}

func (a *accumulator) maxValue() float32 {
	m := float32(0)
	for _, v := range a.Pix {
		if v > m {
			m = v
		}
	}
	return m
}

func cropAccumulator(a *accumulator, r image.Rectangle) *accumulator {
	w, h := r.Dx(), r.Dy()
	out := newAccumulator(w, h)
	for y := 0; y < h; y++ {
		srcOff := ((r.Min.Y+y)*a.Width + r.Min.X) * 3
		dstOff := y * w * 3
		copy(out.Pix[dstOff:dstOff+w*3], a.Pix[srcOff:srcOff+w*3])
	}
	return out
}

func cropCounts(c []int32, w int, r image.Rectangle) []int32 {
	cw, ch := r.Dx(), r.Dy()
	out := make([]int32, cw*ch)
	for y := 0; y < ch; y++ {
		srcOff := (r.Min.Y+y)*w + r.Min.X
		dstOff := y * cw
		copy(out[dstOff:dstOff+cw], c[srcOff:srcOff+cw])
	}
	return out
}

// =========================================================================
// Output — TIFF
// =========================================================================

const (
	tagImageWidth        = 256
	tagImageLength       = 257
	tagBitsPerSample     = 258
	tagCompression       = 259
	tagPhotometricInterp = 262
	tagStripOffsets      = 273
	tagSamplesPerPixel   = 277
	tagRowsPerStrip      = 278
	tagStripByteCounts   = 279
	tagXResolution       = 282
	tagYResolution       = 283
	tagPlanarConfig      = 284
	tagResolutionUnit    = 296
	tagSampleFormat      = 339

	typeShort    = 3
	typeLong     = 4
	typeRational = 5

	compressionNone    = 1
	photometricRGB     = 2
	planarChunky       = 1
	sampleFormatFloat  = 3
	resolutionUnitNone = 1

	ifdEntryCount = 14
)

func writeFloat32RGB(w io.Writer, width, height int, pix []float32) error {
	return writeFloat32RGBScaled(w, width, height, pix, 1.0)
}

func writeRawFloat32RGB(w io.Writer, width, height int, pix []float32) error {
	n := width * height * 3
	if len(pix) != n {
		return fmt.Errorf("raw: buffer mismatch: got %d want %d", len(pix), n)
	}
	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint32(hdr[0:], uint32(width))
	binary.LittleEndian.PutUint32(hdr[4:], uint32(height))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	const chunk = 4096
	buf := make([]byte, chunk*4)
	for i := 0; i < n; {
		count := n - i
		if count > chunk {
			count = chunk
		}
		for j := 0; j < count; j++ {
			binary.LittleEndian.PutUint32(buf[j*4:], math.Float32bits(pix[i+j]))
		}
		if _, err := w.Write(buf[:count*4]); err != nil {
			return err
		}
		i += count
	}
	return nil
}

func readRawFloat32RGB(path string) (int, int, []float32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, nil, err
	}
	defer f.Close()
	hdr := make([]byte, 8)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return 0, 0, nil, fmt.Errorf("raw header: %w", err)
	}
	w := int(binary.LittleEndian.Uint32(hdr[0:]))
	h := int(binary.LittleEndian.Uint32(hdr[4:]))
	n := w * h * 3
	pix := make([]float32, n)
	const chunk = 4096
	buf := make([]byte, chunk*4)
	for i := 0; i < n; {
		count := n - i
		if count > chunk {
			count = chunk
		}
		if _, err := io.ReadFull(f, buf[:count*4]); err != nil {
			return 0, 0, nil, fmt.Errorf("raw data: %w", err)
		}
		for j := 0; j < count; j++ {
			pix[i+j] = math.Float32frombits(binary.LittleEndian.Uint32(buf[j*4:]))
		}
		i += count
	}
	return w, h, pix, nil
}

func writeFloat32RGBScaled(w io.Writer, width, height int, pix []float32, divisor float32) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("tiff: invalid dimensions %dx%d", width, height)
	}
	n := width * height * 3
	if len(pix) != n {
		return fmt.Errorf("tiff: pixel buffer length %d does not match %dx%dx3 = %d", len(pix), width, height, n)
	}
	if divisor == 0 {
		return errors.New("tiff: divisor must be non-zero")
	}
	stripBytes64 := int64(n) * 4
	if stripBytes64 > math.MaxUint32 {
		return fmt.Errorf("tiff: image too large for single strip (%d bytes)", stripBytes64)
	}
	stripBytes := uint32(stripBytes64)

	ifdSize := uint32(2 + ifdEntryCount*12 + 4)
	ifdOffset := uint32(8)
	auxOffset := ifdOffset + ifdSize
	bitsOffset := auxOffset
	sfOffset := bitsOffset + 6
	xResOffset := sfOffset + 6
	yResOffset := xResOffset + 8
	auxEnd := yResOffset + 8
	pixelOffset := auxEnd
	if rem := pixelOffset % 4; rem != 0 {
		pixelOffset += 4 - rem
	}

	var header [8]byte
	header[0], header[1] = 'I', 'I'
	binary.LittleEndian.PutUint16(header[2:], 42)
	binary.LittleEndian.PutUint32(header[4:], ifdOffset)
	if _, err := w.Write(header[:]); err != nil {
		return err
	}

	ifd := make([]byte, ifdSize)
	binary.LittleEndian.PutUint16(ifd[0:], ifdEntryCount)
	pos := 2
	putShort := func(tag, v uint16) {
		binary.LittleEndian.PutUint16(ifd[pos:], tag)
		binary.LittleEndian.PutUint16(ifd[pos+2:], typeShort)
		binary.LittleEndian.PutUint32(ifd[pos+4:], 1)
		binary.LittleEndian.PutUint16(ifd[pos+8:], v)
		pos += 12
	}
	putLong := func(tag uint16, v uint32) {
		binary.LittleEndian.PutUint16(ifd[pos:], tag)
		binary.LittleEndian.PutUint16(ifd[pos+2:], typeLong)
		binary.LittleEndian.PutUint32(ifd[pos+4:], 1)
		binary.LittleEndian.PutUint32(ifd[pos+8:], v)
		pos += 12
	}
	putOffset := func(tag, typ uint16, count, offset uint32) {
		binary.LittleEndian.PutUint16(ifd[pos:], tag)
		binary.LittleEndian.PutUint16(ifd[pos+2:], typ)
		binary.LittleEndian.PutUint32(ifd[pos+4:], count)
		binary.LittleEndian.PutUint32(ifd[pos+8:], offset)
		pos += 12
	}
	putLong(tagImageWidth, uint32(width))
	putLong(tagImageLength, uint32(height))
	putOffset(tagBitsPerSample, typeShort, 3, bitsOffset)
	putShort(tagCompression, compressionNone)
	putShort(tagPhotometricInterp, photometricRGB)
	putLong(tagStripOffsets, pixelOffset)
	putShort(tagSamplesPerPixel, 3)
	putLong(tagRowsPerStrip, uint32(height))
	putLong(tagStripByteCounts, stripBytes)
	putOffset(tagXResolution, typeRational, 1, xResOffset)
	putOffset(tagYResolution, typeRational, 1, yResOffset)
	putShort(tagPlanarConfig, planarChunky)
	putShort(tagResolutionUnit, resolutionUnitNone)
	putOffset(tagSampleFormat, typeShort, 3, sfOffset)
	binary.LittleEndian.PutUint32(ifd[pos:], 0)
	if _, err := w.Write(ifd); err != nil {
		return err
	}

	aux := make([]byte, int(auxEnd-auxOffset))
	bo := int(bitsOffset - auxOffset)
	sfo := int(sfOffset - auxOffset)
	xro := int(xResOffset - auxOffset)
	yro := int(yResOffset - auxOffset)
	binary.LittleEndian.PutUint16(aux[bo:], 32)
	binary.LittleEndian.PutUint16(aux[bo+2:], 32)
	binary.LittleEndian.PutUint16(aux[bo+4:], 32)
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint16(aux[sfo+i*2:], sampleFormatFloat)
	}
	binary.LittleEndian.PutUint32(aux[xro:], 1)
	binary.LittleEndian.PutUint32(aux[xro+4:], 1)
	binary.LittleEndian.PutUint32(aux[yro:], 1)
	binary.LittleEndian.PutUint32(aux[yro+4:], 1)
	if _, err := w.Write(aux); err != nil {
		return err
	}
	if pad := int(pixelOffset) - int(auxEnd); pad > 0 {
		if _, err := w.Write(make([]byte, pad)); err != nil {
			return err
		}
	}

	const bufFloats = 4096
	scratch := make([]byte, bufFloats*4)
	inv := float32(1.0) / divisor
	for i := 0; i < n; {
		count := n - i
		if count > bufFloats {
			count = bufFloats
		}
		for j := 0; j < count; j++ {
			binary.LittleEndian.PutUint32(scratch[j*4:], math.Float32bits(pix[i+j]*inv))
		}
		if _, err := w.Write(scratch[:count*4]); err != nil {
			return err
		}
		i += count
	}
	return nil
}

func readFloat32RGB(r io.Reader) (width, height int, pix []float32, err error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, 0, nil, err
	}
	if len(data) < 8 || data[0] != 'I' || data[1] != 'I' {
		return 0, 0, nil, errors.New("tiff: not little-endian")
	}
	if v := binary.LittleEndian.Uint16(data[2:]); v != 42 {
		return 0, 0, nil, fmt.Errorf("tiff: version %d", v)
	}
	ifdOff := int(binary.LittleEndian.Uint32(data[4:]))
	if ifdOff+2 > len(data) {
		return 0, 0, nil, errors.New("tiff: IFD out of range")
	}
	nEntries := int(binary.LittleEndian.Uint16(data[ifdOff:]))
	if ifdOff+2+nEntries*12+4 > len(data) {
		return 0, 0, nil, errors.New("tiff: IFD truncated")
	}
	var (
		w, h     uint32
		bps, sf  []uint16
		spp      uint16
		photo    uint16
		planar   uint16
		stripOff []uint32
		stripLen []uint32
	)
	for i := 0; i < nEntries; i++ {
		off := ifdOff + 2 + i*12
		tag := binary.LittleEndian.Uint16(data[off:])
		typ := binary.LittleEndian.Uint16(data[off+2:])
		count := int(binary.LittleEndian.Uint32(data[off+4:]))
		valField := off + 8
		readShorts := func() ([]uint16, error) {
			if typ != typeShort {
				return nil, fmt.Errorf("tiff: tag %d type %d", tag, typ)
			}
			out := make([]uint16, count)
			if count*2 <= 4 {
				for k := 0; k < count; k++ {
					out[k] = binary.LittleEndian.Uint16(data[valField+k*2:])
				}
			} else {
				o := int(binary.LittleEndian.Uint32(data[valField:]))
				if o+count*2 > len(data) {
					return nil, errors.New("tiff: offset out of range")
				}
				for k := 0; k < count; k++ {
					out[k] = binary.LittleEndian.Uint16(data[o+k*2:])
				}
			}
			return out, nil
		}
		readLongs := func() ([]uint32, error) {
			if typ != typeLong {
				return nil, fmt.Errorf("tiff: tag %d type %d", tag, typ)
			}
			out := make([]uint32, count)
			if count*4 <= 4 {
				out[0] = binary.LittleEndian.Uint32(data[valField:])
			} else {
				o := int(binary.LittleEndian.Uint32(data[valField:]))
				if o+count*4 > len(data) {
					return nil, errors.New("tiff: offset out of range")
				}
				for k := 0; k < count; k++ {
					out[k] = binary.LittleEndian.Uint32(data[o+k*4:])
				}
			}
			return out, nil
		}
		switch tag {
		case tagImageWidth:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			w = v[0]
		case tagImageLength:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			h = v[0]
		case tagBitsPerSample:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			bps = v
		case tagPhotometricInterp:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			photo = v[0]
		case tagStripOffsets:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			stripOff = v
		case tagSamplesPerPixel:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			spp = v[0]
		case tagStripByteCounts:
			v, e := readLongs()
			if e != nil {
				return 0, 0, nil, e
			}
			stripLen = v
		case tagPlanarConfig:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			planar = v[0]
		case tagSampleFormat:
			v, e := readShorts()
			if e != nil {
				return 0, 0, nil, e
			}
			sf = v
		}
	}
	if w == 0 || h == 0 || spp != 3 || photo != photometricRGB || planar != planarChunky {
		return 0, 0, nil, errors.New("tiff: unexpected header fields")
	}
	if len(bps) != 3 || bps[0] != 32 || len(sf) != 3 || sf[0] != sampleFormatFloat {
		return 0, 0, nil, errors.New("tiff: unexpected sample format")
	}
	if len(stripOff) != 1 || len(stripLen) != 1 {
		return 0, 0, nil, errors.New("tiff: expected single strip")
	}
	nPix := int(w) * int(h) * 3
	if int(stripLen[0]) != nPix*4 {
		return 0, 0, nil, fmt.Errorf("tiff: strip byte count %d, expected %d", stripLen[0], nPix*4)
	}
	off := int(stripOff[0])
	if off+nPix*4 > len(data) {
		return 0, 0, nil, errors.New("tiff: pixel data out of range")
	}
	pix = make([]float32, nPix)
	for i := range pix {
		pix[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[off+i*4:]))
	}
	return int(w), int(h), pix, nil
}

// =========================================================================
// Output — preview JPEG
// =========================================================================

const previewGamma = 0.5
const previewLUTSize = 4096

var previewLUT = buildPreviewLUT()

func buildPreviewLUT() [previewLUTSize + 1]uint8 {
	var lut [previewLUTSize + 1]uint8
	for i := 0; i <= previewLUTSize; i++ {
		v := float64(i) / previewLUTSize
		s := math.Pow(v, previewGamma)
		x := int(s*255 + 0.5)
		if x < 0 {
			x = 0
		} else if x > 255 {
			x = 255
		}
		lut[i] = uint8(x)
	}
	return lut
}

func writePreviewJPEG(w io.Writer, width, height int, pix []float32) error {
	return writePreviewJPEGScaled(w, width, height, pix, 1.0)
}

func writePreviewJPEGScaled(w io.Writer, width, height int, pix []float32, divisor float32) error {
	if divisor == 0 {
		divisor = 1.0
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	inv := float32(1.0) / divisor
	for y := 0; y < height; y++ {
		rowOff := y * img.Stride
		for x := 0; x < width; x++ {
			i := (y*width + x) * 3
			o := rowOff + x*4
			img.Pix[o+0] = previewLUT[stretchIndex(pix[i+0]*inv)]
			img.Pix[o+1] = previewLUT[stretchIndex(pix[i+1]*inv)]
			img.Pix[o+2] = previewLUT[stretchIndex(pix[i+2]*inv)]
			img.Pix[o+3] = 255
		}
	}
	return jpeg.Encode(w, img, &jpeg.Options{Quality: 90})
}

func writeAutoNormalizedPreviewJPEG(w io.Writer, width, height int, pix []float32) error {
	maxVal := float32(0)
	for _, v := range pix {
		if v > maxVal {
			maxVal = v
		}
	}
	if math.IsNaN(float64(maxVal)) || math.IsInf(float64(maxVal), 0) || maxVal <= 0 {
		maxVal = 1.0
	}
	return writePreviewJPEGScaled(w, width, height, pix, maxVal)
}

func stretchIndex(v float32) int {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return previewLUTSize
	}
	return int(v * previewLUTSize)
}

// =========================================================================
// Report
// =========================================================================

type rejection struct {
	Path   string
	Reason string
}

type exifData struct {
	OK               bool
	ExposureTime     float64
	FNumber          float64
	ISO              int
	DateTimeOriginal string
	Make             string
	Model            string
}

func readJPEGExif(path string) exifData {
	e := exifData{}
	f, err := os.Open(path)
	if err != nil {
		return e
	}
	defer f.Close()
	buf := make([]byte, 128*1024)
	n, _ := io.ReadFull(f, buf)
	buf = buf[:n]
	if len(buf) < 4 || buf[0] != 0xFF || buf[1] != 0xD8 {
		return e
	}
	pos := 2
	for pos+4 <= len(buf) {
		if buf[pos] != 0xFF {
			return e
		}
		marker := buf[pos+1]
		if marker == 0xDA || marker == 0xD9 {
			break
		}
		segLen := int(buf[pos+2])<<8 | int(buf[pos+3])
		if segLen < 2 || pos+2+segLen > len(buf) {
			break
		}
		if marker == 0xE1 {
			data := buf[pos+4 : pos+2+segLen]
			if len(data) >= 6 && string(data[:4]) == "Exif" {
				parseExifTIFF(data[6:], &e)
				return e
			}
		}
		pos += 2 + segLen
	}
	return e
}
func parseExifTIFF(data []byte, e *exifData) {
	if len(data) < 8 {
		return
	}
	var be bool
	switch string(data[:2]) {
	case "II":
		be = false
	case "MM":
		be = true
	default:
		return
	}
	u16 := func(off int) uint16 {
		if off+2 > len(data) {
			return 0
		}
		if be {
			return binary.BigEndian.Uint16(data[off:])
		}
		return binary.LittleEndian.Uint16(data[off:])
	}
	u32 := func(off int) uint32 {
		if off+4 > len(data) {
			return 0
		}
		if be {
			return binary.BigEndian.Uint32(data[off:])
		}
		return binary.LittleEndian.Uint32(data[off:])
	}
	if u16(2) != 42 {
		return
	}
	ifdOff := int(u32(4))
	if ifdOff+2 > len(data) {
		return
	}
	n := int(u16(ifdOff))
	if ifdOff+2+n*12 > len(data) {
		return
	}
	readStr := func(count, off int) string {
		if count <= 0 || count > 1024 {
			return ""
		}
		var raw []byte
		if count <= 4 {
			raw = data[off : off+count]
		} else {
			o := int(u32(off))
			if o+count > len(data) {
				return ""
			}
			raw = data[o : o+count]
		}
		for len(raw) > 0 && (raw[len(raw)-1] == 0 || raw[len(raw)-1] == ' ') {
			raw = raw[:len(raw)-1]
		}
		return string(raw)
	}
	readRat := func(off int) float64 {
		o := int(u32(off))
		if o+8 > len(data) {
			return 0
		}
		var num, den uint32
		if be {
			num = binary.BigEndian.Uint32(data[o:])
			den = binary.BigEndian.Uint32(data[o+4:])
		} else {
			num = binary.LittleEndian.Uint32(data[o:])
			den = binary.LittleEndian.Uint32(data[o+4:])
		}
		if den == 0 {
			return 0
		}
		return float64(num) / float64(den)
	}
	exifIfd := -1
	for i := 0; i < n; i++ {
		off := ifdOff + 2 + i*12
		tag := u16(off)
		typ := u16(off + 2)
		count := int(u32(off + 4))
		valOff := off + 8
		switch tag {
		case 0x010F:
			if typ == 2 {
				e.Make = readStr(count, valOff)
			}
		case 0x0110:
			if typ == 2 {
				e.Model = readStr(count, valOff)
			}
		case 0x8769:
			exifIfd = int(u32(valOff))
		}
	}
	if exifIfd > 0 && exifIfd+2 <= len(data) {
		n2 := int(u16(exifIfd))
		if exifIfd+2+n2*12 <= len(data) {
			for i := 0; i < n2; i++ {
				off := exifIfd + 2 + i*12
				tag := u16(off)
				typ := u16(off + 2)
				count := int(u32(off + 4))
				valOff := off + 8
				switch tag {
				case 0x829A:
					if typ == 5 {
						e.ExposureTime = readRat(valOff)
					}
				case 0x829D:
					if typ == 5 {
						e.FNumber = readRat(valOff)
					}
				case 0x8827:
					if typ == 3 {
						e.ISO = int(u16(valOff))
					}
				case 0x9003:
					if typ == 2 {
						e.DateTimeOriginal = readStr(count, valOff)
					}
				}
			}
		}
	}
	e.OK = e.ExposureTime > 0 || e.Make != "" || e.Model != ""
}

type processingReport struct {
	Version            string
	Timestamp          time.Time
	InputDir           string
	OutputDir          string
	Discovered         int
	Accepted           []string
	Rejected           []rejection
	Width              int
	Height             int
	Channels           int
	Method             string
	AccumulatorType    string
	OutputFormat       string
	OutputScaling      string
	RegistrationOn     bool
	RegisteredCount    int
	RegistrationFails  []string
	CropRect           image.Rectangle
	SumNormalization   float32
	PostProcessApplied bool
	BGDegree           int
	StretchFactor      float64
	SharpenApplied     bool
	SharpenAmount      float64
	SharpenRadius      float64
	OutputFiles        []string
	Workers            int
	Duration           time.Duration
	Notes              []string
	FrameDiags         []frameDiag
	CropExplanation    string
	TotalExposureSec   float64
	FramesWithExif     int
	RefExif            exifData
}

type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, args ...any) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e.w, format, args...)
}

func (r *processingReport) writeTo(w io.Writer) error {
	ew := &errWriter{w: w}
	ew.printf("SkyStacker %s\n", r.Version)
	ew.printf("Generated: %s\n\n", r.Timestamp.Format(time.RFC3339))
	ew.printf("Input directory:  %s\n", r.InputDir)
	ew.printf("Output directory: %s\n\n", r.OutputDir)
	ew.printf("Discovered files: %d\n", r.Discovered)
	ew.printf("Accepted:         %d\n", len(r.Accepted))
	ew.printf("Rejected:         %d\n\n", len(r.Rejected))
	ew.printf("Image dimensions: %d x %d\n", r.Width, r.Height)
	ew.printf("Channels:         %d\n\n", r.Channels)
	ew.printf("Method:           %s\n", r.Method)
	ew.printf("Accumulator:      %s\n", r.AccumulatorType)
	ew.printf("Output format:    %s\n", r.OutputFormat)
	ew.printf("Output scaling:   %s\n", r.OutputScaling)
	ew.printf("Workers:          %d\n", r.Workers)
	ew.printf("Duration:         %s\n\n", r.Duration.Round(time.Millisecond))

	ew.printf("Registration:        %v\n", r.RegistrationOn)
	ew.printf("Registered frames:   %d / %d\n", r.RegisteredCount, len(r.Accepted))
	if r.RegistrationOn && r.RegisteredCount > 0 {
		ew.printf("Crop rectangle:      x=%d..%d, y=%d..%d (%dx%d)\n",
			r.CropRect.Min.X, r.CropRect.Max.X-1,
			r.CropRect.Min.Y, r.CropRect.Max.Y-1,
			r.CropRect.Dx(), r.CropRect.Dy())
	}
	if len(r.RegistrationFails) > 0 {
		ew.printf("Registration failures:\n")
		for _, f := range r.RegistrationFails {
			ew.printf("  - %s\n", f)
		}
	}
	ew.printf("\n")

	ew.printf("Sum TIFF normalisation: %g\n", r.SumNormalization)
	if r.TotalExposureSec > 0 || r.RefExif.OK {
		ew.printf("Exposure summary:\n")
		ew.printf("  Frames with EXIF:  %d / %d\n", r.FramesWithExif, len(r.Accepted))
		if r.TotalExposureSec > 0 {
			ew.printf("  Total exposure:    %.3f s (%.2f min)\n", r.TotalExposureSec, r.TotalExposureSec/60)
			if r.FramesWithExif > 0 {
				ew.printf("  Average per frame: %.4f s\n", r.TotalExposureSec/float64(r.FramesWithExif))
			}
		}
		if r.RefExif.OK {
			if r.RefExif.Make != "" || r.RefExif.Model != "" {
				ew.printf("  Camera:            %s %s\n", r.RefExif.Make, r.RefExif.Model)
			}
			if r.RefExif.FNumber > 0 {
				ew.printf("  Aperture:          f/%.1f\n", r.RefExif.FNumber)
			}
			if r.RefExif.ISO > 0 {
				ew.printf("  ISO:               %d\n", r.RefExif.ISO)
			}
			if r.RefExif.DateTimeOriginal != "" {
				ew.printf("  Reference date:    %s\n", r.RefExif.DateTimeOriginal)
			}
		}
		ew.printf("\n")
	}
	if r.PostProcessApplied {
		ew.printf("Post-processing:    ON\n")
		ew.printf("  Background removal: polynomial degree %d\n", r.BGDegree)
		ew.printf("  Contrast stretch:   asinh f=%g\n", r.StretchFactor)
		ew.printf("  Sharpening:         %v (amount=%g, radius=%g)\n",
			r.SharpenApplied, r.SharpenAmount, r.SharpenRadius)
	} else {
		ew.printf("Post-processing:    OFF\n")
	}
	ew.printf("\n")

	ew.printf("Output files:\n")
	for _, f := range r.OutputFiles {
		ew.printf("  %s\n", f)
	}
	ew.printf("\n")

	if len(r.FrameDiags) > 0 {
		ew.printf("Frame diagnostics (per-frame quality, sorted by quality descending):\n")
		sorted := make([]frameDiag, len(r.FrameDiags))
		copy(sorted, r.FrameDiags)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Quality > sorted[j].Quality
		})
		ew.printf("  %-3s %-22s %5s %4s %6s %8s %8s %16s %7s %7s %7s %22s %7s %s\n",
			"#", "File", "Stars", "Inl", "Ratio", "Scale", "Rot", "Offset", "Quality", "IntrQ", "Weight", "Box(x,y,w,h)", "Box%", "Note")
		for i, d := range sorted {
			ratioStr := "-"
			if d.Stars > 0 {
				ratioStr = fmt.Sprintf("%.0f%%", 100.0*float64(d.Inliers)/float64(d.Stars))
			}
			boxStr := fmt.Sprintf("%d,%d,%d,%d", d.BoxMinX, d.BoxMinY, d.BoxMaxX-d.BoxMinX, d.BoxMaxY-d.BoxMinY)
			if d.Stars == 0 {
				boxStr = "-"
			}
			ew.printf("  %-3d %-22s %5d %4d %6s %8.4f %8.2f %16s %7.1f %7.1f %7.2f %22s %6.1f%% %s\n",
				i+1, filepath.Base(d.Path), d.Stars, d.Inliers, ratioStr,
				d.Scale, d.RotationDeg,
				fmt.Sprintf("(%.1f,%.1f)", d.OffsetX, d.OffsetY),
				d.Quality, d.IntrinsicQ, d.Weight, boxStr, d.BoxPct, d.Note)
		}
		ew.printf("\n")
	}

	if r.CropExplanation != "" {
		ew.printf("Crop explanation:\n")
		ew.printf("  %s\n\n", r.CropExplanation)
	}

	ew.printf("Accepted files:\n")
	for _, p := range r.Accepted {
		ew.printf("  %s\n", p)
	}
	ew.printf("\n")
	if len(r.Rejected) > 0 {
		ew.printf("Rejected files:\n")
		for _, rj := range r.Rejected {
			ew.printf("  %s: %s\n", rj.Path, rj.Reason)
		}
		ew.printf("\n")
	}
	ew.printf("Notes:\n")
	for _, n := range r.Notes {
		ew.printf("  - %s\n", n)
	}
	return ew.err
}

// =========================================================================
// Atomic file write
// =========================================================================

func writeFileAtomic(path string, fn func(io.Writer) error) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, 1<<16)
	if err := fn(bw); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := bw.Flush(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// =========================================================================
// Pipeline
// =========================================================================

type runOptions struct {
	Clean                bool
	InputDir             string
	OutputDir            string
	Workers              int
	Recursive            bool
	Overwrite            bool
	Register             bool
	DarkFile             string
	FlatFile             string
	BiasFile             string
	CalibRoot            string
	Method               string
	MaskBG               bool
	PostProcess          bool
	StackOnly            bool
	PostOnly             bool
	NoColorCalib         bool
	NoSharpen            bool
	BGDegree             int
	StretchFactor        float64
	SharpenAmount        float64
	SharpenRadius        float64
	HDRAmount            float64
	StarRadius           int
	StarSigma            float64
	BgPercentile         float64
	NoAutoStretch        bool
	STFShadowsClip       float64
	STFTargetBG          float64
	STFLinked            bool
	NoHDRBlend           bool
	HDRBlendFactor       float64
	NoHDRMix             bool
	NoDenoise            bool
	DenoiseStrength      float64
	DenoiseRadius        int
	DenoiseChroma        float64
	NoCurves             bool
	CurvesBlack          float64
	CurvesGamma          float64
	AddSpikes            bool
	SpikesPercent        float64
	SpikesLength         float64
	SpikesIntensity      float64
	SpikesArms           int
	SpikesSoftness       float64
	SpikesMinRatio       float64
	SpikesLengthSoftness float64
	SpikesLengthMinRatio float64
	SpikesMinCount       int
	NoExif               bool
	SigmaClipThreshold   float64
	SigmaClipIters       int
	TileSize             int
	Scale                int
	Force                bool
	Normalize            bool
	LocalNormalize       bool
	LNTileSize           int

	CreateTIFF  bool
	PostCA      bool
	PostColor   bool
	PostFlatten bool
	PostSharpen bool
	PostHDR     bool
	PostStretch bool

	NoCache  bool
	CacheDir string
}

func stackCalibrationFolder(path string, targetW, targetH int) (*frame, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return decodeImageFile(path, targetW, targetH)
	}
	files, err := discoverFiles(path, false, nil)
	if err != nil || len(files) == 0 {
		return nil, nil // No files found, skip
	}
	logf("  Stacking calibration frames from %s (%d files)", filepath.Base(path), len(files))
	var master *frame
	count := 0
	for _, f := range files {
		img, err := decodeImageFile(f, targetW, targetH)
		if err != nil {
			continue
		}
		if master == nil {
			master = &frame{Width: img.Width, Height: img.Height, Pix: make([]float32, len(img.Pix))}
		}
		for i := range img.Pix {
			master.Pix[i] += img.Pix[i]
		}
		count++
	}
	if count > 0 {
		for i := range master.Pix {
			master.Pix[i] /= float32(count)
		}
	}
	return master, nil
}

func getAvailableMemoryMB() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemAvailable:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.Atoi(fields[1])
				return kb / 1024
			}
		}
	}
	return 0
}

// getAvailableDiskBytes returns the free bytes available to the current
// user on the filesystem containing the given path. It uses `df` on
// Unix and PowerShell on Windows so the entire program remains a
// single cross-platform file with no build tags or platform-specific
// syscalls.
func getAvailableDiskBytes(path string) (int64, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return 0, err
	}
	if runtime.GOOS == "windows" {
		vol := filepath.VolumeName(absPath) + "\\"
		cmd := exec.Command("powershell", "-NoProfile", "-Command",
			fmt.Sprintf("(Get-PSDrive -PSProvider FileSystem | Where-Object { $_.Root -eq '%s' }).Free", vol))
		out, err := cmd.Output()
		if err != nil {
			return 0, fmt.Errorf("powershell: %w", err)
		}
		freeStr := strings.TrimSpace(string(out))
		free, err := strconv.ParseInt(freeStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse powershell output: %w", err)
		}
		return free, nil
	}
	cmd := exec.Command("df", "-P", "-k", absPath)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("df: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0, errors.New("df output too short")
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return 0, fmt.Errorf("df output malformed: %q", lines[1])
	}
	freeKB, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse df output: %w", err)
	}
	return freeKB * 1024, nil
}

// estimateDiskDemand returns the estimated peak disk usage of a stack
// run, in bytes, for the given method and frame count. Pass 1 and Pass 2
// spill one frame each per source frame; the 500 MB headroom covers the
// work file and post-processing outputs. Mean has no spill, so the
// demand is 0 (only RAM matters for streaming accumulation).
func estimateDiskDemand(method string, frameCount int, frameBytes int64) int64 {
	if method != "median" && method != "sigma" {
		return 0
	}
	return int64(frameCount)*frameBytes*2 + 500*1024*1024
}

// getAvailableDiskBytes returns the free bytes for the user on the given
// path's filesystem. Returns (0, err) if statfs is unavailable; callers
// should treat (0, non-nil) as "cannot check, proceed".

// normStats describes a frame's background level and noise: the median
// luminance and the median absolute deviation (MAD) of luminance around
// that median. Two such frames can be aligned by scale = ref.MAD/f.MAD
// and offset = ref.Median - scale*f.Median. Chosen because both are
// robust to the small number of bright pixels (stars, galaxies) that
// dominate the mean but not the median.
type normStats struct {
	Median float64
	MAD    float64
}

// computeRegionStats returns the (median, MAD) pair for the given
// rectangle of f, ignoring masked pixels (r < 0). The rectangle is
// clamped to the frame bounds. Uses a 1024-bin histogram with 4x
// subsampling; quantisation error is ~1/1024, far below sky-glow
// variation.
func computeRegionStats(f *frame, x0, y0, x1, y1 int) normStats {
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > f.Width {
		x1 = f.Width
	}
	if y1 > f.Height {
		y1 = f.Height
	}
	if x1 <= x0 || y1 <= y0 {
		return normStats{}
	}
	const numBins = 1024
	const subsample = 4
	var hist [numBins]int64
	total := int64(0)
	for y := y0; y < y1; y += subsample {
		for x := x0; x < x1; x += subsample {
			i := (y*f.Width + x) * 3
			if f.Pix[i] < 0 {
				continue
			}
			lum := 0.299*f.Pix[i] + 0.587*f.Pix[i+1] + 0.114*f.Pix[i+2]
			if lum < 0 {
				lum = 0
			} else if lum >= 1 {
				lum = 0.999999
			}
			hist[int(lum*numBins)]++
			total++
		}
	}
	if total == 0 {
		return normStats{}
	}
	medianIdx := total / 2
	cum := int64(0)
	var medianBin int
	for i := 0; i < numBins; i++ {
		cum += hist[i]
		if cum >= medianIdx {
			medianBin = i
			break
		}
	}
	median := (float64(medianBin) + 0.5) / float64(numBins)
	var absHist [numBins]int64
	absTotal := int64(0)
	for y := y0; y < y1; y += subsample {
		for x := x0; x < x1; x += subsample {
			i := (y*f.Width + x) * 3
			if f.Pix[i] < 0 {
				continue
			}
			lum := float64(0.299*f.Pix[i] + 0.587*f.Pix[i+1] + 0.114*f.Pix[i+2])
			dev := math.Abs(lum - median)
			if dev >= 1 {
				dev = 0.999999
			}
			absHist[int(dev*numBins)]++
			absTotal++
		}
	}
	if absTotal == 0 {
		return normStats{Median: median, MAD: 0}
	}
	madIdx := absTotal / 2
	cum = 0
	var madBin int
	for i := 0; i < numBins; i++ {
		cum += absHist[i]
		if cum >= madIdx {
			madBin = i
			break
		}
	}
	mad := (float64(madBin) + 0.5) / float64(numBins)
	return normStats{Median: median, MAD: mad}
}

// computeNormStats returns the (median, MAD) pair for the whole frame.
func computeNormStats(f *frame) normStats {
	return computeRegionStats(f, 0, 0, f.Width, f.Height)
}

// applyNormTransform applies value = scale*value + offset to every
// valid pixel, clamping to [0, 1]. Masked pixels (r < 0) stay masked.
func applyNormTransform(f *frame, scale, offset float64) *frame {
	out := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	nPix := f.Width * f.Height
	s := float32(scale)
	o := float32(offset)
	for p := 0; p < nPix; p++ {
		i := p * 3
		if f.Pix[i] < 0 {
			out.Pix[i] = -1
			out.Pix[i+1] = -1
			out.Pix[i+2] = -1
			continue
		}
		for c := 0; c < 3; c++ {
			v := f.Pix[i+c]*s + o
			if v < 0 {
				v = 0
			} else if v > 1 {
				v = 1
			}
			out.Pix[i+c] = v
		}
	}
	return out
}

// globalNormalizeWithRef scales f so its background median and MAD
// match the reference. Frames with degenerate stats are returned
// unchanged; nothing useful can be computed from them.
func globalNormalizeWithRef(f *frame, ref normStats) *frame {
	fs := computeNormStats(f)
	if fs.MAD < 1e-6 || ref.MAD < 1e-6 {
		return f
	}
	scale := ref.MAD / fs.MAD
	if scale < 0.5 {
		scale = 0.5
	}
	if scale > 2.0 {
		scale = 2.0
	}
	offset := ref.Median - scale*fs.Median
	return applyNormTransform(f, scale, offset)
}

// localNormalizeWithRefTiles applies tile-local scale+offset corrections
// with bilinear interpolation of the correction field. refTiles is a
// (nty x ntx) grid of (median, MAD) pairs for the reference frame,
// pre-computed once before the pass-2 loop. Degenerate tiles (MAD near
// 0) fall back to the supplied refGlobal stats so the field stays
// coherent instead of injecting scale=1 discontinuities.
func localNormalizeWithRefTiles(f *frame, refTiles []normStats, refGlobal normStats, ntx, nty, ts int) *frame {
	if ntx <= 0 || nty <= 0 || len(refTiles) != ntx*nty {
		return globalNormalizeWithRef(f, refGlobal)
	}
	if ts < 64 {
		ts = 64
	}
	frameStats := make([]normStats, ntx*nty)
	for ty := 0; ty < nty; ty++ {
		for tx := 0; tx < ntx; tx++ {
			x0 := tx * ts
			y0 := ty * ts
			x1 := x0 + ts
			y1 := y0 + ts
			if x1 > f.Width {
				x1 = f.Width
			}
			if y1 > f.Height {
				y1 = f.Height
			}
			frameStats[ty*ntx+tx] = computeRegionStats(f, x0, y0, x1, y1)
		}
	}
	frameGlobal := computeNormStats(f)
	globalScale := 1.0
	globalOffset := 0.0
	if frameGlobal.MAD > 1e-6 && refGlobal.MAD > 1e-6 {
		globalScale = refGlobal.MAD / frameGlobal.MAD
		if globalScale < 0.5 {
			globalScale = 0.5
		}
		if globalScale > 2.0 {
			globalScale = 2.0
		}
		globalOffset = refGlobal.Median - globalScale*frameGlobal.Median
	}
	scales := make([]float64, ntx*nty)
	offsets := make([]float64, ntx*nty)
	for i := range scales {
		rs := refTiles[i]
		fs := frameStats[i]
		if fs.MAD < 1e-6 || rs.MAD < 1e-6 {
			scales[i] = globalScale
			offsets[i] = globalOffset
			continue
		}
		s := rs.MAD / fs.MAD
		if s < 0.5 {
			s = 0.5
		}
		if s > 2.0 {
			s = 2.0
		}
		scales[i] = s
		offsets[i] = rs.Median - s*fs.Median
	}
	out := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	nPix := f.Width * f.Height
	maxFx := float64(ntx - 1)
	maxFy := float64(nty - 1)
	for p := 0; p < nPix; p++ {
		i := p * 3
		if f.Pix[i] < 0 {
			out.Pix[i] = -1
			out.Pix[i+1] = -1
			out.Pix[i+2] = -1
			continue
		}
		x := p % f.Width
		y := p / f.Width
		fx := (float64(x)+0.5)/float64(ts) - 0.5
		fy := (float64(y)+0.5)/float64(ts) - 0.5
		if fx < 0 {
			fx = 0
		}
		if fy < 0 {
			fy = 0
		}
		if fx > maxFx {
			fx = maxFx
		}
		if fy > maxFy {
			fy = maxFy
		}
		tx0 := int(fx)
		ty0 := int(fy)
		tx1 := tx0 + 1
		ty1 := ty0 + 1
		if tx1 >= ntx {
			tx1 = ntx - 1
		}
		if ty1 >= nty {
			ty1 = nty - 1
		}
		dx := fx - float64(tx0)
		dy := fy - float64(ty0)
		s00 := scales[ty0*ntx+tx0]
		s10 := scales[ty0*ntx+tx1]
		s01 := scales[ty1*ntx+tx0]
		s11 := scales[ty1*ntx+tx1]
		o00 := offsets[ty0*ntx+tx0]
		o10 := offsets[ty0*ntx+tx1]
		o01 := offsets[ty1*ntx+tx0]
		o11 := offsets[ty1*ntx+tx1]
		sc := (1-dx)*(1-dy)*s00 + dx*(1-dy)*s10 + (1-dx)*dy*s01 + dx*dy*s11
		of := (1-dx)*(1-dy)*o00 + dx*(1-dy)*o10 + (1-dx)*dy*o01 + dx*dy*o11
		s := float32(sc)
		o := float32(of)
		for c := 0; c < 3; c++ {
			v := f.Pix[i+c]*s + o
			if v < 0 {
				v = 0
			} else if v > 1 {
				v = 1
			}
			out.Pix[i+c] = v
		}
	}
	return out
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// startProgress spawns a background goroutine that logs a "still running"
// message every interval until the returned stop function is called.
// Intended for stages (tiled reduction, long guided-filter passes) that
// can run for minutes with no other output, so users do not think the
// process is stuck.
//
// Call pattern:
//
//	stop := startProgress("reduce", 15*time.Second)
//	defer stop()
//	...long work...
func startProgress(label string, interval time.Duration) (stop func()) {
	done := make(chan struct{})
	start := time.Now()
	var once sync.Once
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				logf("  [%s] still running, %.0fs elapsed", label, time.Since(start).Seconds())
			}
		}
	}()
	return func() { once.Do(func() { close(done) }) }
}

type frameTransform struct {
	sim similarity
	aff *affine // optional full-affine refinement; when non-nil,
	// the resampler and bounding-box diagnostic use it
	// instead of sim. Set only when affine refinement
	// measurably reduces the inlier residual, so most
	// frames stay on the faster similarity path.
	valid     bool
	starCount int
	inliers   int
}

// frameLimiterFlags classifies a registered frame as a crop limiter
// or low-quality contributor. Returns zero or more of:
//
//   - "LOW QUALITY":  combined registration score < 30 (typically
//     fitted on noise stars, so it does not really
//     constrain the crop).
//   - "LOW BOX%":     transformed bounding box covers < 50% of the
//     reference area.
//   - "LARGE OFFSET": |OffsetX| > 10% of reference width, or
//     |OffsetY| > 10% of reference height. Catches
//     pure-translation drift (e.g. the M42 run's
//     677 px left-side loss) which the bounding box
//     alone misses: a shifted but otherwise full
//     frame still covers most of the reference, yet
//     it drags the safe crop in the shifted
//     direction. Naming these frames lets the user
//     remove or re-align them.
const cacheVersion = 1

type starCache struct {
	Version      int     `json:"version"`
	FrameMtime   int64   `json:"frame_mtime_nano"`
	FrameSize    int64   `json:"frame_size"`
	DetectionSig string  `json:"detection_sig"`
	Stars        []star  `json:"stars"`
	MedianBlob   float64 `json:"median_blob"`
	BgNoise      float64 `json:"bg_noise"`
}

type xfCache struct {
	Version       int        `json:"version"`
	RefMtime      int64      `json:"ref_mtime_nano"`
	HasTransform  bool       `json:"has_transform"`
	Sim           similarity `json:"sim"`
	HasAffine     bool       `json:"has_affine"`
	Affine        affine     `json:"affine"`
	Inliers       int        `json:"inliers"`
	RegFail       bool       `json:"reg_fail"`
	RegFailReason string     `json:"reg_fail_reason"`
}

func frameCacheKey(path string) (key string, mtimeNano, size int64, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", 0, 0, err
	}
	mtimeNano = fi.ModTime().UnixNano()
	size = fi.Size()
	h := sha256.New()
	io.WriteString(h, path)
	io.WriteString(h, "|")
	io.WriteString(h, strconv.FormatInt(mtimeNano, 10))
	io.WriteString(h, "|")
	io.WriteString(h, strconv.FormatInt(size, 10))
	return hex.EncodeToString(h.Sum(nil)), mtimeNano, size, nil
}

func starDetectionSig(scale int) string {
	return fmt.Sprintf("max=%d,scale=%d", 100, scale)
}

func writeJSON(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}

func readJSON(path string, v any) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, v) == nil
}

func loadStarCache(path, expectSig string) (starCache, bool) {
	var sc starCache
	if !readJSON(path, &sc) {
		return starCache{}, false
	}
	if sc.Version != cacheVersion || sc.DetectionSig != expectSig {
		return starCache{}, false
	}
	return sc, true
}

func loadXfCache(path string, expectRefMtime int64) (xfCache, bool) {
	var xc xfCache
	if !readJSON(path, &xc) {
		return xfCache{}, false
	}
	if xc.Version != cacheVersion || xc.RefMtime != expectRefMtime {
		return xfCache{}, false
	}
	return xc, true
}

func loadOrDetectStars(path string, f *frame, cacheDir string, scale int) ([]star, float64, float64, bool) {
	if cacheDir != "" {
		if key, mtime, size, err := frameCacheKey(path); err == nil {
			cpath := filepath.Join(cacheDir, fmt.Sprintf("stars_s%d_%s.json", scale, key))
			if sc, ok := loadStarCache(cpath, starDetectionSig(scale)); ok {
				return sc.Stars, sc.MedianBlob, sc.BgNoise, true
			}
			stars := detectStars(f, 100)
			mb := medianBlobSize(stars)
			bn := estimateFrameBgNoise(f)
			sc := starCache{
				Version: cacheVersion, FrameMtime: mtime, FrameSize: size,
				DetectionSig: starDetectionSig(scale),
				Stars:        stars, MedianBlob: mb, BgNoise: bn,
			}
			if err := writeJSON(cpath, sc); err != nil {
				logf("  WARNING: could not write star cache for %s: %v", filepath.Base(path), err)
			}
			return stars, mb, bn, false
		}
	}
	stars := detectStars(f, 100)
	return stars, medianBlobSize(stars), estimateFrameBgNoise(f), false
}

func checkXfCache(cacheDir, refPath, framePath string, scale int) (xfCache, bool) {
	if cacheDir == "" {
		return xfCache{}, false
	}
	refKey, refMtime, _, err := frameCacheKey(refPath)
	if err != nil {
		return xfCache{}, false
	}
	frameKey, _, _, err := frameCacheKey(framePath)
	if err != nil {
		return xfCache{}, false
	}
	xpath := filepath.Join(cacheDir, fmt.Sprintf("xf_s%d_%s_%s.json", scale, refKey, frameKey))
	return loadXfCache(xpath, refMtime)
}

func writeXfCacheFail(cacheDir, refPath, framePath string, scale int, reason string) {
	if cacheDir == "" {
		return
	}
	refKey, refMtime, _, err := frameCacheKey(refPath)
	if err != nil {
		return
	}
	frameKey, _, _, err := frameCacheKey(framePath)
	if err != nil {
		return
	}
	xpath := filepath.Join(cacheDir, fmt.Sprintf("xf_s%d_%s_%s.json", scale, refKey, frameKey))
	xc := xfCache{Version: cacheVersion, RefMtime: refMtime, RegFail: true, RegFailReason: reason}
	if err := writeJSON(xpath, xc); err != nil {
		logf("  WARNING: could not write xf cache for %s: %v", filepath.Base(framePath), err)
	}
}

func writeXfCacheOK(cacheDir, refPath, framePath string, scale int, sim similarity, affRef *affine, inliers int) {
	if cacheDir == "" {
		return
	}
	refKey, refMtime, _, err := frameCacheKey(refPath)
	if err != nil {
		return
	}
	frameKey, _, _, err := frameCacheKey(framePath)
	if err != nil {
		return
	}
	xpath := filepath.Join(cacheDir, fmt.Sprintf("xf_s%d_%s_%s.json", scale, refKey, frameKey))
	xc := xfCache{Version: cacheVersion, RefMtime: refMtime, HasTransform: true, Sim: sim, Inliers: inliers}
	if affRef != nil {
		xc.HasAffine = true
		xc.Affine = *affRef
	}
	if err := writeJSON(xpath, xc); err != nil {
		logf("  WARNING: could not write xf cache for %s: %v", filepath.Base(framePath), err)
	}
}

func frameLimiterFlags(d frameDiag, refW, refH int) []string {
	var flags []string
	if d.Quality < 30 {
		flags = append(flags, "LOW QUALITY")
	}
	if d.BoxPct < 50 {
		flags = append(flags, "LOW BOX%")
	}
	// The raw OffsetX/Y is where the frame's top-left corner lands after
	// the similarity. On a rotation-only similarity about the frame
	// center, that corner inevitably moves even when there is no real
	// drift. Subtract the rotation-induced component so the check only
	// fires on genuine translation drift (e.g. the M42 677 px shift),
	// not on every legitimately-rotated frame (e.g. Polaris at 25 deg).
	//
	// The s <= 0 || s > 10 guard keeps backward compatibility with
	// frameDiag values built without Scale set (zero value): without
	// it, expX/expY would be (cx, cy) instead of (0, 0) and every
	// existing test fixture would start failing.
	cx, cy := float64(refW)/2, float64(refH)/2
	s := d.Scale
	if s <= 0 || s > 10 {
		s = 1.0
	}
	theta := d.RotationDeg * math.Pi / 180
	cosT, sinT := math.Cos(theta), math.Sin(theta)
	expX := cx*(1-s*cosT) + s*cy*sinT
	expY := -s*cx*sinT + cy*(1-s*cosT)
	driftX := d.OffsetX - expX
	driftY := d.OffsetY - expY
	if math.Abs(driftX) > 0.10*float64(refW) || math.Abs(driftY) > 0.10*float64(refH) {
		flags = append(flags, "LARGE OFFSET")
	}
	return flags
}

// frameDiag carries per-frame diagnostics for the report: detected stars,
// matched inliers, the estimated transform (scale/rotation/offset), a
// combined quality score (0-100), the transformed bounding box in
// reference coordinates, and the fraction of the reference area that box
// covers (Box%). Used to explain the crop rectangle and to spot frames
// that should be removed.
type frameDiag struct {
	Path        string
	Stars       int
	Inliers     int
	Scale       float64
	RotationDeg float64
	OffsetX     float64
	OffsetY     float64
	Quality     float64
	BoxMinX     int
	BoxMinY     int
	BoxMaxX     int
	BoxMaxY     int
	BoxPct      float64 // 100 * box area / reference area
	BgNoise     float64
	MedianBlob  float64
	IntrinsicQ  float64
	Weight      float64
	Note        string
}

// computeFrameQuality returns a 0-100 score combining inlier ratio, a
// penalty for scale deviation from 1.0, and a penalty for rotation away
// from 0 degrees. Frames scoring below ~30 are strong candidates for
// removal (typically because they were registered on noise).
func computeFrameQuality(inliers, stars, refStars int, scale, rotDeg float64) float64 {
	minCount := stars
	if refStars < minCount {
		minCount = refStars
	}
	if minCount == 0 {
		return 0
	}
	ratio := float64(inliers) / float64(minCount)
	if ratio > 1 {
		ratio = 1
	}
	q := 100.0 * ratio
	if math.Abs(scale-1.0) > 0.05 {
		q *= 0.5
	}
	if math.Abs(rotDeg) > 5.0 {
		q *= 0.5
	}
	return q
}

// computeIntrinsicQuality returns a 0-100 score based only on the frame's
// own metrics: star count, median connected-component size (a proxy for
// FWHM / focus quality), and background noise sigma. Independent of
// registration, so it can score frames even when the reference-frame
// registration pass cannot use them.
//
// Components (0-100 total):
//
//	starScore   0-40: 0 at 0 stars, 40 at 100+ stars
//	blobScore   0-30: 30 at median Size <= 3 px (sharp), 0 at >= 10 px
//	noiseScore  0-30: 30 at bg noise <= 0.01, 0 at >= 0.15
func computeIntrinsicQuality(starCount int, medianBlob, bgNoise float64) float64 {
	if starCount == 0 {
		return 0
	}
	starScore := math.Min(float64(starCount)/100.0, 1.0) * 40.0
	blobScore := 30.0
	if medianBlob > 3.0 {
		blobScore = math.Max(0, 30.0*(1.0-(medianBlob-3.0)/7.0))
	}
	noiseScore := 30.0
	if bgNoise > 0.01 {
		noiseScore = math.Max(0, 30.0*(1.0-(bgNoise-0.01)/0.14))
	}
	return starScore + blobScore + noiseScore
}

// medianBlobSize returns the median of the Size field across all stars.
// A larger median Size correlates with blurrier stars. Returns 0 if no
// stars were detected.
func medianBlobSize(stars []star) float64 {
	if len(stars) == 0 {
		return 0
	}
	sizes := make([]float64, len(stars))
	for i, s := range stars {
		sizes[i] = float64(s.Size)
	}
	sort.Float64s(sizes)
	return sizes[len(sizes)/2]
}

// estimateFrameBgNoise returns the MAD-based background noise sigma of the
// luminance channel, using the same high-pass radius as detectStarsSigma.
// Cheap O(N) pass; intended to be run once per frame in pass 1.
func estimateFrameBgNoise(f *frame) float64 {
	gray := grayscale(f)
	hpRadius := estimateHighPassRadius(f.Width, f.Height)
	blurred := boxBlurLuminance(gray, f.Width, f.Height, hpRadius)
	residual := make([]float32, len(gray))
	for i := range gray {
		residual[i] = gray[i] - blurred[i]
	}
	return estimateResidualNoise(residual)
}

// sigmaClipReduceWeighted is the weighted variant of sigmaClipReduce.
// The MAD-based active-mask computation is identical; the weighted mean
// of the surviving values is returned. If all surviving weights are 0,
// the unweighted mean is returned as a safe fallback so pixels are not
// zeroed by a division-by-zero.
func sigmaClipReduceWeighted(vals, weights []float32, threshold float64, iters int) (mean, sigma float32, accepted, rejected int) {
	n := len(vals)
	if n != len(weights) || n == 0 {
		return 0, 0, 0, 0
	}
	if threshold <= 0 {
		threshold = 3.0
	}
	if iters < 1 {
		iters = 1
	}
	active := make([]bool, n)
	for i := range active {
		active[i] = true
	}
	var lastMedian float32
	for it := 0; it < iters; it++ {
		var activeVals []float32
		for i, a := range active {
			if a {
				activeVals = append(activeVals, vals[i])
			}
		}
		if len(activeVals) < 3 {
			var s, ws float64
			for i, a := range active {
				if a {
					s += float64(vals[i]) * float64(weights[i])
					ws += float64(weights[i])
				}
			}
			if ws <= 1e-12 {
				for i, a := range active {
					if a {
						s += float64(vals[i])
						ws += 1.0
					}
				}
			}
			if ws <= 1e-12 {
				return 0, 0, 0, n
			}
			return float32(s / ws), 0, len(activeVals), n - len(activeVals)
		}
		med := medianInPlace(activeVals)
		lastMedian = med
		mad := madOf(activeVals, med)
		sig := 1.4826 * mad
		if sig < 1e-9 {
			sig = 1e-9
		}
		sigma = sig
		k := float32(threshold * float64(sig))
		anyActive := false
		for i, a := range active {
			if !a {
				continue
			}
			d := vals[i] - med
			if d < 0 {
				d = -d
			}
			if d > k {
				active[i] = false
			} else {
				anyActive = true
			}
		}
		if !anyActive {
			return lastMedian, sig, 1, n - 1
		}
	}
	var s, ws float64
	for i, a := range active {
		if a {
			s += float64(vals[i]) * float64(weights[i])
			ws += float64(weights[i])
			accepted++
		} else {
			rejected++
		}
	}
	if ws <= 1e-12 {
		var us float64
		for i, a := range active {
			if a {
				us += float64(vals[i])
			}
		}
		if accepted == 0 {
			return lastMedian, sigma, 1, n - 1
		}
		return float32(us / float64(accepted)), sigma, accepted, rejected
	}
	return float32(s / ws), sigma, accepted, rejected
}

// frameBoxInReference returns the axis-aligned bounding box of a frame
// of size fw x fh after its four corners have been mapped through sim
// into the reference coordinate system. For an identity sim, the box is
// (0, 0, fw-1, fh-1). Rotated frames get a larger box.
func frameBoxInReference(fw, fh int, sim similarity) (minX, minY, maxX, maxY int) {
	pts := []point2D{
		{0, 0},
		{float64(fw - 1), 0},
		{float64(fw - 1), float64(fh - 1)},
		{0, float64(fh - 1)},
	}
	minX, minY = math.MaxInt32, math.MaxInt32
	maxX, maxY = -1, -1
	for _, p := range pts {
		q := sim.apply(p)
		x, y := int(q.X), int(q.Y)
		if x < minX {
			minX = x
		}
		if y < minY {
			minY = y
		}
		if x > maxX {
			maxX = x
		}
		if y > maxY {
			maxY = y
		}
	}
	return minX, minY, maxX, maxY
}

// frameBoxInReferenceAffine is the affine variant of frameBoxInReference.
// Use it when registration was refined to a full affine model.
func frameBoxInReferenceAffine(fw, fh int, aff affine) (minX, minY, maxX, maxY int) {
	pts := []point2D{
		{0, 0},
		{float64(fw - 1), 0},
		{float64(fw - 1), float64(fh - 1)},
		{0, float64(fh - 1)},
	}
	minX, minY = math.MaxInt32, math.MaxInt32
	maxX, maxY = -1, -1
	for _, p := range pts {
		q := aff.apply(p)
		x, y := int(q.X), int(q.Y)
		if x < minX {
			minX = x
		}
		if y < minY {
			minY = y
		}
		if x > maxX {
			maxX = x
		}
		if y > maxY {
			maxY = y
		}
	}
	return minX, minY, maxX, maxY
}

// medianInPlace sorts vals ascending in place and returns the median
// (average of the two central values for even lengths). Safe for n=0.
func medianInPlace(vals []float32) float32 {
	n := len(vals)
	if n == 0 {
		return 0
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	if n%2 == 0 {
		return 0.5 * (vals[n/2-1] + vals[n/2])
	}
	return vals[n/2]
}

// madOf returns the median absolute deviation of vals given the
// precomputed median. It does not modify vals.
func madOf(vals []float32, median float32) float32 {
	dev := make([]float32, len(vals))
	for i, v := range vals {
		d := v - median
		if d < 0 {
			d = -d
		}
		dev[i] = d
	}
	return medianInPlace(dev)
}

// sigmaClipReduce returns the MAD-based sigma-clipped mean of vals. The
// first iteration computes median and MAD, derives sigma = 1.4826*MAD,
// and rejects values further than threshold*sigma from the median. Each
// subsequent iteration recomputes from the surviving values. If fewer
// than 3 values survive, the mean of those is returned unchanged.
func sigmaClipReduce(vals []float32, threshold float64, iters int) (mean, sigma float32, accepted, rejected int) {
	n := len(vals)
	if n == 0 {
		return 0, 0, 0, 0
	}
	if threshold <= 0 {
		threshold = 3.0
	}
	if iters < 1 {
		iters = 1
	}
	active := make([]bool, n)
	for i := range active {
		active[i] = true
	}
	var lastMedian float32
	for it := 0; it < iters; it++ {
		var activeVals []float32
		for i, a := range active {
			if a {
				activeVals = append(activeVals, vals[i])
			}
		}
		if len(activeVals) < 3 {
			var s float64
			for _, v := range activeVals {
				s += float64(v)
			}
			if len(activeVals) == 0 {
				return 0, 0, 0, n
			}
			return float32(s / float64(len(activeVals))), 0, len(activeVals), n - len(activeVals)
		}
		med := medianInPlace(activeVals)
		lastMedian = med
		mad := madOf(activeVals, med)
		sig := 1.4826 * mad
		if sig < 1e-9 {
			sig = 1e-9
		}
		sigma = sig
		k := float32(threshold * float64(sig))
		anyActive := false
		for i, a := range active {
			if !a {
				continue
			}
			d := vals[i] - med
			if d < 0 {
				d = -d
			}
			if d > k {
				active[i] = false
			} else {
				anyActive = true
			}
		}
		if !anyActive {
			return lastMedian, sig, 1, n - 1
		}
	}
	var s float64
	for i, a := range active {
		if a {
			s += float64(vals[i])
			accepted++
		} else {
			rejected++
		}
	}
	if accepted == 0 {
		return lastMedian, sigma, 1, n - 1
	}
	return float32(s / float64(accepted)), sigma, accepted, rejected
}

// writeSpillFrame writes a frame's interleaved RGB float32 buffer to disk
// as a raw file with an 8-byte header (uint32 width, uint32 height). The
// format matches readRawFloat32RGB so the existing reader works.
func writeSpillFrame(path string, f *frame) error {
	return writeFileAtomic(path, func(w io.Writer) error {
		return writeRawFloat32RGB(w, f.Width, f.Height, f.Pix)
	})
}

// readSpillFrame reads a full raw float32 frame into a fresh *frame.
func readSpillFrame(path string, w, h int) (*frame, error) {
	gw, gh, pix, err := readRawFloat32RGB(path)
	if err != nil {
		return nil, err
	}
	if gw != w || gh != h {
		return nil, fmt.Errorf("spill frame dims %dx%d != %dx%d", gw, gh, w, h)
	}
	return &frame{Width: w, Height: h, Pix: pix}, nil
}

// readTileFromRaw reads a rectangular tile from a raw float32 RGB file
// into dst. dst must be at least (x1-x0)*(y1-y0)*3 float32s. The tile is
// written to dst in row-major interleaved RGB order (same layout as a
// decoded frame's Pix).
func readTileFromRaw(path string, fullW, fullH, x0, y0, x1, y1 int, dst []float32) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hdr := make([]byte, 8)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return err
	}
	gw := int(binary.LittleEndian.Uint32(hdr[0:]))
	gh := int(binary.LittleEndian.Uint32(hdr[4:]))
	if gw != fullW || gh != fullH {
		return fmt.Errorf("raw dims %dx%d != %dx%d", gw, gh, fullW, fullH)
	}
	tileW := x1 - x0
	tileH := y1 - y0
	if len(dst) < tileW*tileH*3 {
		return fmt.Errorf("dst too small: %d < %d", len(dst), tileW*tileH*3)
	}
	rowBytes := int64(fullW) * 3 * 4
	const chunk = 4096
	buf := make([]byte, chunk*4)
	for py := 0; py < tileH; py++ {
		off := int64(8) + int64(y0+py)*rowBytes + int64(x0)*3*4
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return err
		}
		nFloats := tileW * 3
		done := 0
		for done < nFloats {
			c := nFloats - done
			if c > chunk {
				c = chunk
			}
			if _, err := io.ReadFull(f, buf[:c*4]); err != nil {
				return err
			}
			base := py*tileW*3 + done
			for j := 0; j < c; j++ {
				dst[base+j] = math.Float32frombits(binary.LittleEndian.Uint32(buf[j*4:]))
			}
			done += c
		}
	}
	return nil
}

// updateValidCount increments validCount[p] for every pixel in f whose
// red channel is >= 0. The same rule as accumulator.addFrameMasked.
func updateValidCount(f *frame, validCount []int32) {
	nPix := len(validCount)
	for p := 0; p < nPix; p++ {
		if f.Pix[p*3] >= 0 {
			validCount[p]++
		}
	}
}

// reduceTilesStack performs the tiled median / sigma-clipped-mean reduction
// over a set of spilled raw float32 frames. Each spill file must have
// dimensions fullW x fullH. crop selects the output region (in full-frame
// coordinates). Returns:
//   - sumPix:  per-pixel sum of valid values, output-sized (outW*outH*3)
//   - meanPix: reduction result (median or clipped mean), output-sized
//   - rejPix:  per-pixel rejected count, averaged across RGB (outW*outH)
//   - wgtPix:  per-pixel accepted count, averaged across RGB (outW*outH)
func reduceTilesStack(
	spillPaths []string,
	fullW, fullH int,
	crop image.Rectangle,
	method string,
	sigmaThreshold float64,
	sigmaIters int,
	tileSize int,
	workers int,
	frameWeights []float32,
) (sumPix, meanPix, rejPix, wgtPix []float32, err error) {
	outW := crop.Dx()
	outH := crop.Dy()
	nFrames := len(spillPaths)
	if nFrames == 0 {
		return nil, nil, nil, nil, errors.New("no spill frames")
	}
	nPix := outW * outH
	sumPix = make([]float32, nPix*3)
	meanPix = make([]float32, nPix*3)
	rejPix = make([]float32, nPix)
	wgtPix = make([]float32, nPix)

	ts := tileSize
	if ts < 64 {
		ts = 256
	}
	// Cap tile buffer to ~256 MB.
	const maxTileBuf = 256 * 1024 * 1024
	maxSide := int(math.Sqrt(float64(maxTileBuf / (nFrames * 3 * 4))))
	if maxSide < 64 {
		maxSide = 64
	}
	if ts > maxSide {
		ts = maxSide
	}

	type tile struct {
		x0, y0, x1, y1 int
	}
	var tiles []tile
	for y := 0; y < outH; y += ts {
		for x := 0; x < outW; x += ts {
			x1 := x + ts
			if x1 > outW {
				x1 = outW
			}
			y1 := y + ts
			if y1 > outH {
				y1 = outH
			}
			tiles = append(tiles, tile{x, y, x1, y1})
		}
	}

	jobs := make(chan tile, len(tiles))
	for _, t := range tiles {
		jobs <- t
	}
	close(jobs)

	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once
	if workers < 1 {
		workers = 1
	}
	if workers > len(tiles) {
		workers = len(tiles)
	}

	var completedTiles int64
	totalTiles := int64(len(tiles))
	reportEvery := totalTiles / 20
	if reportEvery < 5 {
		reportEvery = 5
	}
	startRed := time.Now()
	stopTicker := startProgress(method+" reduce", 15*time.Second)
	defer stopTicker()

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range jobs {
				if e := reduceTile(spillPaths, fullW, fullH, crop, t.x0, t.y0, t.x1, t.y1,
					method, sigmaThreshold, sigmaIters, outW, sumPix, meanPix, rejPix, wgtPix, frameWeights); e != nil {
					errOnce.Do(func() { firstErr = e })
					for range jobs {
					}
					return
				}
				if n := atomic.AddInt64(&completedTiles, 1); n%int64(reportEvery) == 0 || n == totalTiles {
					logf("  reduce: %d/%d tiles (%.1f%%, %.0fs elapsed)", n, totalTiles, 100.0*float64(n)/float64(totalTiles), time.Since(startRed).Seconds())
				}
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, nil, nil, nil, firstErr
	}
	return
}

func reduceTile(spillPaths []string, fullW, fullH int, crop image.Rectangle,
	x0, y0, x1, y1 int, method string,
	sigmaThreshold float64, sigmaIters int,
	outW int, sumPix, meanPix, rejPix, wgtPix []float32, frameWeights []float32) error {
	tileW := x1 - x0
	tileH := y1 - y0
	nFrames := len(spillPaths)
	nPix := tileW * tileH

	// buf[pix*3*nFrames + c*nFrames + fi] so per-pixel per-channel gather
	// is contiguous.
	buf := make([]float32, nPix*3*nFrames)
	frameTile := make([]float32, nPix*3)

	fullX0 := crop.Min.X + x0
	fullY0 := crop.Min.Y + y0

	for fi, path := range spillPaths {
		if err := readTileFromRaw(path, fullW, fullH, fullX0, fullY0, fullX0+tileW, fullY0+tileH, frameTile); err != nil {
			return fmt.Errorf("read tile from %s: %w", path, err)
		}
		for py := 0; py < tileH; py++ {
			for px := 0; px < tileW; px++ {
				pix := py*tileW + px
				src := pix * 3
				base := pix*3*nFrames + fi
				for c := 0; c < 3; c++ {
					buf[base+c*nFrames] = frameTile[src+c]
				}
			}
		}
	}

	validVals := make([]float32, 0, nFrames)
	validWeights := make([]float32, 0, nFrames)
	workVals := make([]float32, 0, nFrames)

	for py := 0; py < tileH; py++ {
		for px := 0; px < tileW; px++ {
			pix := py*tileW + px
			outPix := (y0+py)*outW + (x0 + px)
			var rejSum, wgtSum float32
			for c := 0; c < 3; c++ {
				off := pix*3*nFrames + c*nFrames
				vals := buf[off : off+nFrames]
				validVals = validVals[:0]
				validWeights = validWeights[:0]
				for fi, v := range vals {
					if v >= 0 {
						validVals = append(validVals, v)
						if frameWeights != nil {
							validWeights = append(validWeights, frameWeights[fi])
						}
					}
				}
				if len(validVals) == 0 {
					sumPix[outPix*3+c] = 0
					meanPix[outPix*3+c] = 0
					continue
				}
				var s float64
				for _, v := range validVals {
					s += float64(v)
				}
				sumPix[outPix*3+c] = float32(s)
				switch method {
				case "median":
					workVals = workVals[:0]
					workVals = append(workVals, validVals...)
					meanPix[outPix*3+c] = medianInPlace(workVals)
					wgtSum += float32(len(validVals))
				case "sigma":
					var m float32
					var acc, rej int
					if frameWeights != nil {
						m, _, acc, rej = sigmaClipReduceWeighted(validVals, validWeights, sigmaThreshold, sigmaIters)
					} else {
						m, _, acc, rej = sigmaClipReduce(validVals, sigmaThreshold, sigmaIters)
					}
					meanPix[outPix*3+c] = m
					wgtSum += float32(acc)
					rejSum += float32(rej)
				}
			}
			rejPix[outPix] = rejSum / 3.0
			wgtPix[outPix] = wgtSum / 3.0
		}
	}
	return nil
}

// writeCoverageMap writes a per-pixel coverage map: each pixel's value
// is (valid frame count at that pixel) / totalRegistered, clamped to [0,1].
// "Valid" means the resampler produced a real sample there (not -1 from
// resampling outside the source bounds). Full-frame dimensions for the
// registered stack: the map shows where the safe crop was taken from,
// because every pixel inside the crop is 100% covered by construction.
//
// Unlike the rejection/weight maps this map is linear-mapped, not
// auto-normalized, so 0.0 is always black and 1.0 is always white. That
// keeps the meaning stable across runs where the maximum coverage varies.
func writeCoverageMap(outputDir string, w, h int, counts []int32, totalRegistered int, suffix string) {
	if counts == nil || totalRegistered <= 0 {
		return
	}
	if len(counts) < w*h {
		return
	}
	inv := 1.0 / float32(totalRegistered)
	rgb := make([]float32, w*h*3)
	for i := 0; i < w*h; i++ {
		v := float32(counts[i]) * inv
		if v < 0 {
			v = 0
		} else if v > 1 {
			v = 1
		}
		rgb[i*3] = v
		rgb[i*3+1] = v
		rgb[i*3+2] = v
	}
	p := filepath.Join(outputDir, "stacked"+suffix+"_coverage_map.jpg")
	if err := writeFileAtomic(p, func(wr io.Writer) error {
		return writeLinearPreviewJPEG(wr, w, h, rgb)
	}); err != nil {
		logf("  WARNING: coverage map: %v", err)
	}
}

// writeGrayscaleMaps writes the rejection and weight maps as single-channel
// JPEGs next to the stacked outputs. Best effort: failures are logged but
// not fatal.
func writeGrayscaleMaps(outputDir string, w, h int, rej, wgt []float32, suffix string) {
	if rej == nil && wgt == nil {
		return
	}
	rgb := make([]float32, w*h*3)
	if rej != nil {
		for i, v := range rej {
			rgb[i*3] = v
			rgb[i*3+1] = v
			rgb[i*3+2] = v
		}
		p := filepath.Join(outputDir, "stacked"+suffix+"_rejection_map.jpg")
		if err := writeFileAtomic(p, func(wr io.Writer) error {
			return writeAutoNormalizedLinearPreviewJPEG(wr, w, h, rgb)
		}); err != nil {
			logf("  WARNING: rejection map: %v", err)
		}
	}
	if wgt != nil {
		for i, v := range wgt {
			rgb[i*3] = v
			rgb[i*3+1] = v
			rgb[i*3+2] = v
		}
		p := filepath.Join(outputDir, "stacked"+suffix+"_weight_map.jpg")
		if err := writeFileAtomic(p, func(wr io.Writer) error {
			return writeAutoNormalizedLinearPreviewJPEG(wr, w, h, rgb)
		}); err != nil {
			logf("  WARNING: weight map: %v", err)
		}
	}
}

// calibLayout describes the standard calibration folder names found under
// a top-level dataset directory. Each field is an absolute path, or "" if
// no matching subfolder exists. This lets a user point skystacker at
// 04_emission_nebula_Flame/ and have it stack Light/*.fits with the
// matching Dark/, Flat/ and Bias/ frames without needing four flags.
type calibLayout struct {
	Light, Dark, Flat, Bias string
}

// detectCalibrationSubdirs searches inputDir for subfolders with the
// standard calibration names. Both singular and plural forms are
// accepted, case-insensitively. Matching is done by exact folder name
// (Light, Darks, etc.), not by substring, so a folder named "Lighter
// subs" is not mistaken for a Light folder.
func detectCalibrationSubdirs(inputDir string) calibLayout {
	check := func(names ...string) string {
		for _, name := range names {
			p := filepath.Join(inputDir, name)
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				return p
			}
		}
		return ""
	}
	return calibLayout{
		Light: check("Light", "Lights", "LIGHT", "LIGHTS"),
		Dark:  check("Dark", "Darks", "DARK", "DARKS"),
		Flat:  check("Flat", "Flats", "FLAT", "FLATS"),
		Bias:  check("Bias", "Biases", "BIAS", "BIASES"),
	}
}

func runPipeline(ctx context.Context, opts runOptions) error {
	start := time.Now()

	// Tighter GC while we hold large frame buffers.  Default GOGC=100 lets
	// the heap double between collections, which on a sigma/median stack
	// of ~86 frames x ~46 MB easily pushes RSS past the OS limit and gets
	// the process OOM-killed during pass 2.  GOGC=50 halves the slack.
	oldGC := debug.SetGCPercent(50)
	defer debug.SetGCPercent(oldGC)

	inputDir, err := filepath.Abs(opts.InputDir)
	if err != nil {
		return err
	}
	info, err := os.Stat(inputDir)
	if err != nil {
		return fmt.Errorf("input directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("input path is not a directory: %s", inputDir)
	}

	var outputDir string
	if opts.OutputDir == "" {
		outputDir = filepath.Join(inputDir, "0_skystacker_results")
	} else {
		outputDir, err = filepath.Abs(opts.OutputDir)
		if err != nil {
			return err
		}
	}
	if filepath.Clean(outputDir) == filepath.Clean(inputDir) {
		return fmt.Errorf("output directory must differ from input directory")
	}
	if opts.Clean {
		defer os.RemoveAll(filepath.Join(outputDir, "_work"))
	}
	if opts.PostOnly {
		if _, err := os.Stat(outputDir); err != nil {
			return fmt.Errorf("--post-only: output directory does not exist: %w", err)
		}
		meanPath := filepath.Join(outputDir, "_work", "stacked_mean.f32raw")
		if _, err := os.Stat(meanPath); err != nil {
			return fmt.Errorf("--post-only: %s not found: %w", meanPath, err)
		}
		logf("Post-only mode: reading %s", meanPath)
		postOutputs, err := runPostProcessing(outputDir, opts)
		if err != nil {
			return fmt.Errorf("post-processing: %w", err)
		}
		logf("Post-processing complete: %d files", len(postOutputs))
		return nil
	}

	if info, err := os.Stat(outputDir); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("output path exists and is not a directory: %s", outputDir)
		}
		if !opts.Overwrite {
			return fmt.Errorf("output directory already exists: %s (use --overwrite to replace)", outputDir)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("output directory: %w", err)
	}

	// Auto-detect standard calibration subfolders. If inputDir contains a
	// Light/ (or Lights/) subfolder, discovery runs there instead, and any
	// Dark/, Flat/, Bias/ subfolder becomes the corresponding calibration
	// source unless --calib-root or an explicit --dark/--flat/--bias flag
	// is already set.
	//
	// --calib-root takes precedence: when it is set, the user has explicitly
	// described the layout and we leave all detection disabled. Calibration
	// subfolders are always excluded from light-frame discovery so that a
	// recursive scan cannot stack dark frames as lights.
	lightInputDir := inputDir
	excludes := []string{
		outputDir,
		filepath.Join(inputDir, "skystacker_results"),
		filepath.Join(inputDir, "0_skystacker_results"),
	}
	if opts.CalibRoot == "" {
		layout := detectCalibrationSubdirs(inputDir)
		if layout.Light != "" {
			logf("Auto-detected Light/ subfolder: %s", layout.Light)
			lightInputDir = layout.Light
		}
		if opts.DarkFile == "" && layout.Dark != "" {
			logf("Auto-detected Dark/ subfolder: %s", layout.Dark)
			opts.DarkFile = layout.Dark
		}
		if opts.FlatFile == "" && layout.Flat != "" {
			logf("Auto-detected Flat/ subfolder: %s", layout.Flat)
			opts.FlatFile = layout.Flat
		}
		if opts.BiasFile == "" && layout.Bias != "" {
			logf("Auto-detected Bias/ subfolder: %s", layout.Bias)
			opts.BiasFile = layout.Bias
		}
		for _, p := range []string{layout.Dark, layout.Flat, layout.Bias} {
			if p != "" {
				excludes = append(excludes, p)
			}
		}
	}
	files, err := discoverFiles(lightInputDir, opts.Recursive, excludes)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no image files (JPG/PNG/RAW/FITS) found in %s", lightInputDir)
	}

	type candidate struct {
		path       string
		exif       exifData
		bgNoise    float64
		medianBlob float64
	}
	var (
		candidates []candidate
		rejected   []rejection
		refW, refH int
	)

	// Decode dimensions of every file. Files that fail to decode or
	// report invalid dimensions are soft-rejected here and never enter
	// the size-uniformity check below; they are reported in report.txt.
	type dimKey struct{ w, h int }
	type decodedFile struct {
		path string
		rawW int
		rawH int
	}
	decodable := make([]decodedFile, 0, len(files))
	for _, path := range files {
		cfg, err := decodeImageConfig(path)
		if err != nil {
			rejected = append(rejected, rejection{Path: path, Reason: err.Error()})
			continue
		}
		if cfg.Width <= 0 || cfg.Height <= 0 {
			rejected = append(rejected, rejection{Path: path, Reason: fmt.Sprintf("invalid dimensions %dx%d", cfg.Width, cfg.Height)})
			continue
		}
		decodable = append(decodable, decodedFile{path: path, rawW: cfg.Width, rawH: cfg.Height})
	}
	if len(decodable) == 0 {
		for i, rj := range rejected {
			if i >= 5 {
				logf("  ... and %d more rejected (see report.txt)", len(rejected)-5)
				break
			}
			logf("  rejected %s: %s", filepath.Base(rj.Path), rj.Reason)
		}
		return fmt.Errorf("no valid images (all %d candidates rejected)", len(files))
	}

	// Group decodable files by their scaled dimensions.
	refW, refH = decodable[0].rawW/opts.Scale, decodable[0].rawH/opts.Scale
	groups := make(map[dimKey][]string, len(decodable))
	order := make([]dimKey, 0, len(decodable))
	for _, d := range decodable {
		k := dimKey{d.rawW / opts.Scale, d.rawH / opts.Scale}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], d.path)
	}

	// Fail-fast: a single folder that mixes dimensions is almost always
	// a mistake (previews left behind next to the raw subs, a different
	// exposure accidentally copied in, an old output directory that was
	// not excluded). Silently stacking only one subset is the worst
	// possible answer; the user deserves a loud, actionable error.
	if len(order) > 1 {
		var sb strings.Builder
		sb.WriteString("input folder mixes images of different dimensions; all images in the folder must be exactly the same size\n")
		sb.WriteString(fmt.Sprintf("found %d distinct size group(s) among %d decodable file(s):\n", len(order), len(decodable)))
		for _, k := range order {
			names := groups[k]
			var sample []string
			for i, p := range names {
				if i >= 5 {
					sample = append(sample, fmt.Sprintf("... and %d more", len(names)-5))
					break
				}
				sample = append(sample, filepath.Base(p))
			}
			sb.WriteString(fmt.Sprintf("  %dx%d  (%d file(s)): %s\n", k.w, k.h, len(names), strings.Join(sample, ", ")))
		}
		sb.WriteString("Move the mismatching files out of the folder, or resize them to a common size, then retry.")
		return errors.New(sb.String())
	}

	// All decodable files share the same size: register them all.
	for _, d := range decodable {
		candidates = append(candidates, candidate{path: d.path})
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(candidates) {
		workers = len(candidates)
	}
	if workers < 1 {
		workers = 1
	}

	logf("Stacking %d frame(s) at %dx%d using up to %d worker(s)",
		len(candidates), refW, refH, workers)

	// Time estimation. Calibrated against two real sigma runs on
	// 2400x1600 JPGs:
	//    70 frames ->  598 s ( 9 m 58 s)
	//   152 frames -> 1522 s (25 m 22 s)
	//
	// The median/sigma reduction is O(N log N) per pixel, so the total
	// grows super-linearly with frame count. Fitting T = k * N^alpha * MP
	// to the two data points above gives alpha ~= 1.20, k ~= 1.01 for
	// sigma. The old formula (linear in N) was 2.0x low at N=70 and 2.4x
	// low at N=152, which is exactly the growth you'd expect from missing
	// the log factor.
	//
	// Mean has no reduction step (streaming accumulate) and stays linear
	// in N, but its coefficient is corrected upward from 0.35 to 0.55 --
	// the old value was also low, just not super-linear-low.
	//
	// Median sits between mean and sigma: single sort per pixel (O(N log N))
	// but only one pass (no MAD iterations), so alpha=1.18, k=0.88.
	mpx := float64(refW*refH) / 1e6
	nf := float64(len(candidates))
	var estSecs float64
	switch opts.Method {
	case "median":
		estSecs = 0.88 * math.Pow(nf, 1.18) * mpx
	case "sigma":
		estSecs = 1.03 * math.Pow(nf, 1.20) * mpx
	default: // mean
		estSecs = nf * mpx * 0.55
	}
	// CR2/CR3 decode via dcraw + convert adds ~12 s of subprocess
	// overhead per frame, which dominates pixel-level processing for
	// typical sizes.
	for _, c := range candidates {
		ext := strings.ToLower(filepath.Ext(c.path))
		if ext == ".cr2" || ext == ".cr3" {
			estSecs += float64(len(candidates)) * 12.0
			break
		}
	}
	// Post-processing: base ~15 s for loading the work file and writing
	// the ten output JPEGs, plus ~5 s per denoise pass (guided filter is
	// the dominant cost) and an extra 3 s if spike overlay runs its own
	// star detection. The previous flat 10 s was too low for any run with
	// --denoise-strength > 1.
	if opts.PostProcess && !opts.StackOnly {
		estSecs += 15.0
		if !opts.NoDenoise && opts.DenoiseStrength > 0 {
			estSecs += 5.0 * opts.DenoiseStrength
		}
		if opts.AddSpikes {
			estSecs += 3.0
		}
	}
	logf("Estimated processing time: ~%s", time.Duration(estSecs*float64(time.Second)).Round(time.Second))

	// Memory estimation.
	//
	// Mean method (streaming): only one accumulator plus 2 max/min buffers,
	// plus one decoded frame at a time. Peak is ~5 x frameMB.
	//
	// Median/sigma method (out-of-core): frames are spilled to disk, so
	// the only large live buffers are 2 x max/min + validCount +
	// sumPix + meanPix + rejPix/wgtPix + one tile buffer. The tile buffer
	// is capped at 256 MB inside reduceTilesStack.
	frameMB := float64(refW*refH*3*4) / (1024 * 1024)
	var estMemMB int
	var diskMB float64
	if opts.Method == "median" || opts.Method == "sigma" {
		// Pass 1+2 live: max/min (2x), validCount (1x), 1 decoded frame.
		// Reduction live: sumPix + meanPix (2x), rej+wgt (0.67x),
		// tile buffer (capped).
		baseMem := frameMB * 4
		tileBufMB := 256.0
		estMemMB = int(baseMem + tileBufMB)
		// Each frame is spilled twice (pass 1 raw, pass 2 resampled).
		diskMB = float64(len(candidates)) * frameMB * 2.0
		logf("Median/sigma: spilling %d frame(s) to disk (~%.0f MB peak)", len(candidates), diskMB)
	} else {
		estMemMB = int(frameMB * 5)
	}
	availMemMB := getAvailableMemoryMB()

	// Real peak is ~1.5x the naive estimate because Go's GC only triggers
	// when the heap doubles (GOGC=100). We also keep the 2 GB safety margin.
	realPeak := int(float64(estMemMB) * 1.5)
	requiredMB := realPeak + 2000

	logf("Memory: naive estimate %d MB, real peak (GC-adjusted) %d MB, available %d MB",
		estMemMB, realPeak, availMemMB)

	if availMemMB > 0 && requiredMB > availMemMB && !opts.Force {
		hint := "--method mean (streaming, ~200 MB)"
		if opts.Method == "mean" {
			hint = "--scale 2 (halve dimensions, 4x less memory)"
		}
		return fmt.Errorf("OUT OF MEMORY PREVENTED\n"+
			"  Naive estimate:       %d MB\n"+
			"  Real peak (GC lag):   %d MB\n"+
			"  Safety margin:        2000 MB\n"+
			"  Total required:       %d MB\n"+
			"  Currently available:  %d MB\n\n"+
			"Use one of these options:\n"+
			"    %s\n"+
			"    --scale 2       Halve image dimensions (4x less memory)\n"+
			"    --force         Skip this safety check (NOT recommended)\n\n"+
			"Or close memory-hungry apps (Chrome, Firefox, Slack) and retry.",
			estMemMB, realPeak, requiredMB, availMemMB, hint)
	}
	frameBytes := int64(refW) * int64(refH) * 3 * 4
	// Disk pre-flight: median/sigma spill two raw float32 frames per
	// source frame to _work/_spill1 and _work/_spill2. If the output
	// filesystem cannot hold the estimate (with 20% headroom), abort
	// before Pass 1 instead of dying at frame N with ENOSPC after hours
	// of work. --force skips this check, matching the RAM guard.
	if !opts.Force {
		demandBytes := estimateDiskDemand(opts.Method, len(candidates), frameBytes)
		if demandBytes > 0 {
			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return fmt.Errorf("create output directory: %w", err)
			}
			freeBytes, ferr := getAvailableDiskBytes(outputDir)
			if ferr != nil {
				logf("WARNING: disk-space pre-flight skipped: %v", ferr)
			} else {
				requiredBytes := int64(float64(demandBytes) * 1.20)
				if freeBytes < requiredBytes {
					os.Remove(outputDir)
					return fmt.Errorf("INSUFFICIENT DISK SPACE\n"+
						"  Estimated disk demand:  %.2f GB  (%s, %d frames)\n"+
						"  Free on %s:    %.2f GB\n"+
						"  Required (demand+20%%):  %.2f GB\n\n"+
						"Free space on the output filesystem, or use --scale N / --force\n"+
						"    --scale 2    Halve dimensions (4x less spill)\n"+
						"    --force      Skip this safety check (not recommended)",
						float64(demandBytes)/(1024*1024*1024), opts.Method, len(candidates),
						outputDir, float64(freeBytes)/(1024*1024*1024),
						float64(requiredBytes)/(1024*1024*1024))
				}
			}
		}
	}
	if opts.Register {
		logf("Registration: ON (use --no-register to disable)")
	} else {
		logf("Registration: OFF")
	}

	// Star/transform cache. Lives inside <output>/_work/cache so it is
	// cleaned up alongside the rest of the session's intermediate files.
	var cacheDir string
	if !opts.NoCache {
		cdir := opts.CacheDir
		if cdir == "" {
			cdir = filepath.Join(outputDir, "_work", "cache")
		}
		if err := os.MkdirAll(cdir, 0o755); err != nil {
			logf("WARNING: cache disabled, cannot create %s: %v", cdir, err)
		} else {
			cacheDir = cdir
			logf("Cache: %s", cacheDir)
		}
	}
	var starHits, xfHits int

	// Auto-detect calibration frames if --calib-root is provided
	if opts.CalibRoot != "" {
		filterName := filepath.Base(inputDir) // e.g., "Luminance", "Red"
		opts.BiasFile = filepath.Join(opts.CalibRoot, "Bias")
		opts.DarkFile = filepath.Join(opts.CalibRoot, "Darks")
		opts.FlatFile = filepath.Join(opts.CalibRoot, "Flats", filterName)
	}

	var darkFrame, flatFrame, biasFrame *frame
	if opts.DarkFile != "" {
		darkFrame, err = stackCalibrationFolder(opts.DarkFile, refW, refH)
		if err != nil {
			return err
		}
	}
	if opts.FlatFile != "" {
		flatFrame, err = stackCalibrationFolder(opts.FlatFile, refW, refH)
		if err != nil {
			return err
		}
	}
	if opts.BiasFile != "" {
		biasFrame, err = stackCalibrationFolder(opts.BiasFile, refW, refH)
		if err != nil {
			return err
		}
	}

	// ============== Pass 1: decode, detect stars, accumulate unregistered
	unregAcc := newAccumulator(refW, refH)
	var maxPix, minPix []float32
	var spillPaths1 []string
	var spillDir1, spillDir2 string
	defer func() {
		if spillDir1 != "" {
			os.RemoveAll(spillDir1)
		}
		if spillDir2 != "" {
			os.RemoveAll(spillDir2)
		}
	}()
	if opts.Method == "median" || opts.Method == "sigma" {
		spillDir1 = filepath.Join(outputDir, "_work", "_spill1")
		if err := os.MkdirAll(spillDir1, 0o755); err != nil {
			return fmt.Errorf("create spill dir 1: %w", err)
		}
	}
	starLists := make([][]star, len(candidates))
	var accepted []candidate
	for i, c := range candidates {
		if err := ctx.Err(); err != nil {
			return err
		}
		logf("  [pass1 %d/%d] %s", i+1, len(candidates), filepath.Base(c.path))
		f, err := decodeImageFile(c.path, refW, refH)
		if err != nil {
			rejected = append(rejected, rejection{
				Path:   c.path,
				Reason: "decode failed after validation: " + err.Error(),
			})
			continue
		}
		calibrate(f, darkFrame, flatFrame, biasFrame)
		if maxPix == nil {
			maxPix = make([]float32, len(f.Pix))
			minPix = make([]float32, len(f.Pix))
			copy(maxPix, f.Pix)
			copy(minPix, f.Pix)
		} else {
			for j, v := range f.Pix {
				if v > maxPix[j] {
					maxPix[j] = v
				}
				if v < minPix[j] {
					minPix[j] = v
				}
			}
		}

		var frameStars []star
		if opts.Register {
			var wasHit bool
			frameStars, c.medianBlob, c.bgNoise, wasHit = loadOrDetectStars(c.path, f, cacheDir, opts.Scale)
			if wasHit {
				starHits++
				logf("  [cache hit] %s", filepath.Base(c.path))
			}
			starLists[len(accepted)] = frameStars
		}
		if opts.Method == "median" || opts.Method == "sigma" {
			sp := filepath.Join(spillDir1, fmt.Sprintf("frame_%04d.f32raw", len(spillPaths1)))
			if err := writeSpillFrame(sp, f); err != nil {
				return fmt.Errorf("spill %s: %w", c.path, err)
			}
			spillPaths1 = append(spillPaths1, sp)
		}
		// Accumulate into the plain mean accumulator regardless of
		// method. This keeps stacked_no_correction_mean semantically
		// correct (it is the plain arithmetic mean without any
		// registration), while the sigma/median reduction writes
		// its own outputs into unregSumPix / unregMeanPix below.
		if err := unregAcc.addFrame(f, workers); err != nil {
			return fmt.Errorf("accumulate %s: %w", c.path, err)
		}
		f.Pix = nil
		if !opts.NoExif {
			c.exif = readJPEGExif(c.path)
		}
		accepted = append(accepted, c)
	}
	if unregAcc.Count == 0 && len(spillPaths1) == 0 {
		return fmt.Errorf("no frames accumulated")
	}
	if len(spillPaths1) > 0 {
		runtime.GC()
		logf("Spilled %d frame(s) to disk for %s reduction (memory freed)",
			len(spillPaths1), opts.Method)
	}
	starLists = starLists[:len(accepted)]

	// ============== Registration: pick reference, compute transforms
	transforms := make([]frameTransform, len(accepted))
	for i := range transforms {
		transforms[i] = frameTransform{sim: identitySimilarity, valid: true}
	}
	var registeredCount int
	var regFailures []string
	registrationOn := opts.Register
	refIdx := 0
	var refStars []star

	if registrationOn {
		// Reference = frame with the most stars.
		for i := 1; i < len(starLists); i++ {
			if len(starLists[i]) > len(starLists[refIdx]) {
				refIdx = i
			}
		}
		refStars = starLists[refIdx]
		logf("Reference frame: %s (%d stars)", filepath.Base(accepted[refIdx].path), len(refStars))

		if len(refStars) < 5 {
			logf("WARNING: reference frame has only %d stars; disabling registration", len(refStars))
			registrationOn = false
		} else {
			registeredCount = 1
			transforms[refIdx].starCount = len(refStars)
			transforms[refIdx].inliers = len(refStars)
			for i := range accepted {
				if i == refIdx {
					continue
				}
				if len(starLists[i]) < 3 {
					regFailures = append(regFailures, fmt.Sprintf("%s: only %d stars detected",
						filepath.Base(accepted[i].path), len(starLists[i])))
					transforms[i].valid = false
					continue
				}
				// Phase A: transform cache. Skip RANSAC + affine on a hit.
				if xc, hit := checkXfCache(cacheDir, accepted[refIdx].path, accepted[i].path, opts.Scale); hit {
					xfHits++
					if xc.RegFail {
						regFailures = append(regFailures, xc.RegFailReason)
						transforms[i].valid = false
						continue
					}
					if xc.HasTransform {
						tf := frameTransform{
							sim: xc.Sim, valid: true,
							starCount: len(starLists[i]), inliers: xc.Inliers,
						}
						if xc.HasAffine {
							affCopy := xc.Affine
							tf.aff = &affCopy
						}
						transforms[i] = tf
						registeredCount++
						logf("  register %s: [cache hit] scale=%.4f rot=%.2f° inliers=%d",
							filepath.Base(accepted[i].path), xc.Sim.scale(), xc.Sim.rotation()*180/math.Pi, xc.Inliers)
						continue
					}
				}
				sim, inliers, ok := matchStarsToReference(refStars, starLists[i])
				if !ok {
					reason := fmt.Sprintf("%s: no consistent transform (%d inliers)",
						filepath.Base(accepted[i].path), inliers)
					regFailures = append(regFailures, reason)
					transforms[i].valid = false
					writeXfCacheFail(cacheDir, accepted[refIdx].path, accepted[i].path, opts.Scale, reason)
					continue
				}
				scale := sim.scale()
				rotDeg := sim.rotation() * 180 / math.Pi

				// Optional affine refinement. Only attempt when there are
				// enough inliers to make a 6-DOF fit well-conditioned, and
				// only keep it when it measurably reduces the residual.
				var affRef *affine
				if inliers >= 15 {
					srcPts, dstPts := collectInliers(sim, starLists[i], refStars, 3.0)
					if len(srcPts) >= 15 {
						if aff, aok := estimateAffine(srcPts, dstPts); aok {
							if sc := aff.scale(); sc > 0.5 && sc < 2.0 {
								var simRes, affRes float64
								for j := range srcPts {
									pS := sim.apply(srcPts[j])
									pA := aff.apply(srcPts[j])
									simRes += math.Hypot(pS.X-dstPts[j].X, pS.Y-dstPts[j].Y)
									affRes += math.Hypot(pA.X-dstPts[j].X, pA.Y-dstPts[j].Y)
								}
								if simRes > 1e-6 && affRes < 0.90*simRes {
									affCopy := aff
									affRef = &affCopy
									logf("  register %s: scale=%.4f rot=%.2f° inliers=%d [affine %.1f%% residual]",
										filepath.Base(accepted[i].path), scale, rotDeg, inliers,
										100*(1-affRes/simRes))
								}
							}
						}
					}
				}
				if affRef == nil {
					logf("  register %s: scale=%.4f rot=%.2f° inliers=%d",
						filepath.Base(accepted[i].path), scale, rotDeg, inliers)
				}
				writeXfCacheOK(cacheDir, accepted[refIdx].path, accepted[i].path, opts.Scale, sim, affRef, inliers)
				transforms[i] = frameTransform{
					sim:       sim,
					aff:       affRef,
					valid:     true,
					starCount: len(starLists[i]),
					inliers:   inliers,
				}
				registeredCount++
			}
			if cacheDir != "" {
				logf("Cache: %d/%d star hits, %d/%d transform hits", starHits, len(accepted), xfHits, len(accepted)-1)
			}
			if registeredCount < 2 {
				logf("WARNING: only %d frame(s) registered; disabling registration", registeredCount)
				registrationOn = false
			}
		}
	}
	if !registrationOn {
		registeredCount = 0
		for i := range transforms {
			transforms[i] = frameTransform{sim: identitySimilarity, valid: true}
		}
	}
	if !registrationOn && (opts.Normalize || opts.LocalNormalize) {
		logf("WARNING: normalization requires registration; --normalize/--local-normalize ignored")
	}

	// Compute per-frame weights for weighted stacking.
	// Only done when the user requested registration (opts.Register);
	// --no-register runs use equal weights.
	var unregWeights []float32
	var frameWeights []float32
	if opts.Register {
		unregWeights = make([]float32, len(accepted))
		for i, c := range accepted {
			intrinsic := computeIntrinsicQuality(len(starLists[i]), c.medianBlob, c.bgNoise)
			unregWeights[i] = float32(intrinsic / 100.0)
		}
		if registrationOn {
			frameWeights = make([]float32, len(accepted))
			for i, c := range accepted {
				if !transforms[i].valid {
					frameWeights[i] = 0
					continue
				}
				intrinsic := computeIntrinsicQuality(len(starLists[i]), c.medianBlob, c.bgNoise)
				tq := computeFrameQuality(transforms[i].inliers, transforms[i].starCount, len(refStars), transforms[i].sim.scale(), transforms[i].sim.rotation()*180/math.Pi)
				combined := tq * intrinsic / 100.0
				if combined < 0 {
					combined = 0
				}
				if combined > 100 {
					combined = 100
				}
				frameWeights[i] = float32(combined / 100.0)
			}
		}
	}

	// EXIF summary: sum exposures across accepted frames
	var totalExposureSec float64
	var framesWithExif int
	var refExif exifData
	if !opts.NoExif {
		for _, c := range accepted {
			if c.exif.OK {
				framesWithExif++
				if c.exif.ExposureTime > 0 {
					totalExposureSec += c.exif.ExposureTime
				}
			}
		}
		if registrationOn && refIdx < len(accepted) {
			refExif = accepted[refIdx].exif
		}
		if !refExif.OK {
			for _, c := range accepted {
				if c.exif.OK {
					refExif = c.exif
					break
				}
			}
		}
		if totalExposureSec > 0 {
			logf("EXIF: total exposure = %.3f s over %d/%d frames", totalExposureSec, framesWithExif, len(accepted))
		}
	}

	// ============== Pass 2: resample + accumulate registered
	var regAcc *accumulator
	var validCount []int32
	var cropRect image.Rectangle
	var frameDiags []frameDiag
	cropExplanation := ""
	var regSumPix, regMeanPix, regRejPix, regWgtPix []float32
	var spillPaths2 []string
	var needsNormalize bool
	var normRefGlobal normStats
	var normRefTiles []normStats
	var normNTX, normNTY int
	if registrationOn {
		logf("Resampling %d registered frame(s)", registeredCount)
		// Normalization setup (opt-in). Requires registration because the
		// reference frame's background is only meaningful once every frame
		// is expressed in the reference coordinate system.
		needsNormalize = opts.Normalize || opts.LocalNormalize
		if needsNormalize {
			ts := opts.LNTileSize
			if ts < 64 {
				ts = 64
			}
			var refFrame *frame
			var rerr error
			if len(spillPaths1) == len(accepted) {
				refFrame, rerr = readSpillFrame(spillPaths1[refIdx], refW, refH)
			} else {
				refFrame, rerr = decodeImageFile(accepted[refIdx].path, refW, refH)
				if rerr == nil {
					calibrate(refFrame, darkFrame, flatFrame, biasFrame)
				}
			}
			if rerr != nil {
				logf("WARNING: normalization disabled (could not load reference): %v", rerr)
				needsNormalize = false
			} else {
				normRefGlobal = computeNormStats(refFrame)
				if opts.LocalNormalize {
					normNTX = (refW + ts - 1) / ts
					normNTY = (refH + ts - 1) / ts
					normRefTiles = make([]normStats, normNTX*normNTY)
					for ty := 0; ty < normNTY; ty++ {
						for tx := 0; tx < normNTX; tx++ {
							x0 := tx * ts
							y0 := ty * ts
							x1 := x0 + ts
							y1 := y0 + ts
							if x1 > refW {
								x1 = refW
							}
							if y1 > refH {
								y1 = refH
							}
							normRefTiles[ty*normNTX+tx] = computeRegionStats(refFrame, x0, y0, x1, y1)
						}
					}
					logf("Local normalization: %dx%d tiles of %d px (ref median=%.4f, MAD=%.4f)",
						normNTX, normNTY, ts, normRefGlobal.Median, normRefGlobal.MAD)
				} else {
					logf("Global normalization: ref median=%.4f, MAD=%.4f",
						normRefGlobal.Median, normRefGlobal.MAD)
				}
				refFrame.Pix = nil
			}
		}
		regAcc = newAccumulator(refW, refH)
		validCount = make([]int32, refW*refH)
		if opts.Method == "median" || opts.Method == "sigma" {
			spillDir2 = filepath.Join(outputDir, "_work", "_spill2")
			if err := os.MkdirAll(spillDir2, 0o755); err != nil {
				return fmt.Errorf("create spill dir 2: %w", err)
			}
		}
		for i, c := range accepted {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !transforms[i].valid {
				continue
			}
			logf("  [pass2 %d/%d] %s", i+1, len(accepted), filepath.Base(c.path))
			var f *frame
			if len(spillPaths1) == len(accepted) {
				// Fast path: reuse the spilled frame, no re-decode.
				f, err = readSpillFrame(spillPaths1[i], refW, refH)
				if err != nil {
					return fmt.Errorf("reload spill1 %s: %w", c.path, err)
				}
			} else {
				f, err = decodeImageFile(c.path, refW, refH)
				if err != nil {
					return fmt.Errorf("re-decode %s: %w", c.path, err)
				}
			}
			calibrate(f, darkFrame, flatFrame, biasFrame)
			var resampled *frame
			if transforms[i].aff != nil {
				resampled = resampleToReferenceAffine(f, *transforms[i].aff, refW, refH, workers)
			} else {
				resampled = resampleToReference(f, transforms[i].sim, refW, refH, workers)
			}
			f.Pix = nil
			// Normalization (opt-in, requires registration). Applies to
			// every frame except the reference, whose own stats define
			// the target (identity normalization).
			if needsNormalize && i != refIdx {
				if opts.LocalNormalize && len(normRefTiles) > 0 {
					resampled = localNormalizeWithRefTiles(resampled, normRefTiles, normRefGlobal, normNTX, normNTY, opts.LNTileSize)
				} else {
					resampled = globalNormalizeWithRef(resampled, normRefGlobal)
				}
			}
			if opts.Method == "mean" {
				if err := regAcc.addFrameMasked(resampled, workers, validCount); err != nil {
					return fmt.Errorf("accumulate %s: %w", c.path, err)
				}
			} else {
				updateValidCount(resampled, validCount)
				sp := filepath.Join(spillDir2, fmt.Sprintf("frame_%04d.f32raw", len(spillPaths2)))
				if err := writeSpillFrame(sp, resampled); err != nil {
					return fmt.Errorf("spill resampled %s: %w", c.path, err)
				}
				spillPaths2 = append(spillPaths2, sp)
				resampled.Pix = nil
			}
		}
		runtime.GC()
		cropRect = findSafeCrop(validCount, refW, refH, int32(registeredCount))
		logf("Crop: x=%d..%d, y=%d..%d (%dx%d)", cropRect.Min.X, cropRect.Max.X-1,
			cropRect.Min.Y, cropRect.Max.Y-1, cropRect.Dx(), cropRect.Dy())

		// Per-frame diagnostics. refIdx and refStars were hoisted to
		// function scope above so both this pass-2 block and the
		// registration block can see them.
		frameDiags = make([]frameDiag, len(accepted))
		for i := 0; i < len(accepted); i++ {
			td := frameDiag{Path: accepted[i].path}
			if i == refIdx {
				td.Note = "[ref]"
			}
			if !transforms[i].valid {
				td.Note = "REGISTRATION FAILED"
				frameDiags[i] = td
				continue
			}
			td.Stars = transforms[i].starCount
			td.Inliers = transforms[i].inliers
			td.Scale = transforms[i].sim.scale()
			td.RotationDeg = transforms[i].sim.rotation() * 180 / math.Pi
			origin := transforms[i].sim.apply(point2D{0, 0})
			td.OffsetX = origin.X
			td.OffsetY = origin.Y
			td.Quality = computeFrameQuality(td.Inliers, td.Stars, len(refStars), td.Scale, td.RotationDeg)
			td.BgNoise = accepted[i].bgNoise
			td.MedianBlob = accepted[i].medianBlob
			td.IntrinsicQ = computeIntrinsicQuality(td.Stars, td.MedianBlob, td.BgNoise)
			if frameWeights != nil && i < len(frameWeights) {
				td.Weight = float64(frameWeights[i])
			}
			if transforms[i].aff != nil {
				td.Note = strings.TrimSpace(td.Note + " [affine]")
			}
			if transforms[i].aff != nil {
				td.BoxMinX, td.BoxMinY, td.BoxMaxX, td.BoxMaxY = frameBoxInReferenceAffine(refW, refH, *transforms[i].aff)
			} else {
				td.BoxMinX, td.BoxMinY, td.BoxMaxX, td.BoxMaxY = frameBoxInReference(refW, refH, transforms[i].sim)
			}
			bx0, by0 := td.BoxMinX, td.BoxMinY
			bx1, by1 := td.BoxMaxX, td.BoxMaxY
			if bx0 < 0 {
				bx0 = 0
			}
			if by0 < 0 {
				by0 = 0
			}
			if bx1 >= refW {
				bx1 = refW - 1
			}
			if by1 >= refH {
				by1 = refH - 1
			}
			if bx1 >= bx0 && by1 >= by0 {
				td.BoxPct = 100.0 * float64((bx1-bx0+1)*(by1-by0+1)) / float64(refW*refH)
			}
			frameDiags[i] = td
		}

		// Identify limiting frames
		limiterNames := []string{}
		for i := range frameDiags {
			if i == refIdx || !transforms[i].valid {
				continue
			}
			flags := frameLimiterFlags(frameDiags[i], refW, refH)
			if len(flags) > 0 {
				frameDiags[i].Note = strings.TrimSpace(frameDiags[i].Note + " ** " + strings.Join(flags, ", ") + " **")
				limiterNames = append(limiterNames, filepath.Base(frameDiags[i].Path))
			}
		}
		cropPct := 100.0 * float64(cropRect.Dx()*cropRect.Dy()) / float64(refW*refH)
		cropExplanation = fmt.Sprintf(
			"Crop rectangle is %dx%d (%.1f%% of the %dx%d reference). All registered frames cover the full reference image, so no frame is limiting the crop.",
			cropRect.Dx(), cropRect.Dy(), cropPct, refW, refH)
		if len(limiterNames) > 0 {
			logf("Crop limited by %d frame(s): %s", len(limiterNames), strings.Join(limiterNames, ", "))

			// Classify the limiters: translation drift vs field rotation.
			// Translation drift is actionable (remove the drifting frames);
			// field rotation is inherent to the mount and removing those
			// frames trades crop for stacked-frame count. The .Note string
			// is a presentation concern and may be empty when a flag was
			// disabled, so recompute drift and rotation directly.
			anyTranslationDrift := false
			anyFieldRotation := false
			for i := range frameDiags {
				if i == refIdx || !transforms[i].valid {
					continue
				}
				d := frameDiags[i]
				if len(frameLimiterFlags(d, refW, refH)) == 0 {
					continue
				}
				cx, cy := float64(refW)/2, float64(refH)/2
				s := d.Scale
				if s <= 0 || s > 10 {
					s = 1.0
				}
				theta := d.RotationDeg * math.Pi / 180
				cosT, sinT := math.Cos(theta), math.Sin(theta)
				expX := cx*(1-s*cosT) + s*cy*sinT
				expY := -s*cx*sinT + cy*(1-s*cosT)
				driftX := d.OffsetX - expX
				driftY := d.OffsetY - expY
				if math.Abs(driftX) > 0.10*float64(refW) || math.Abs(driftY) > 0.10*float64(refH) {
					anyTranslationDrift = true
				}
				if math.Abs(d.RotationDeg) > 5.0 {
					anyFieldRotation = true
				}
			}

			switch {
			case anyTranslationDrift:
				cropExplanation = fmt.Sprintf(
					"Crop rectangle is %dx%d (%.1f%% of the %dx%d reference). The following frames constrain the crop or have low registration quality: %s. A normal frame has scale~1.0 and rotation~0deg (translation-only alignment); frames with scale far from 1 or rotation far from 0 were typically fitted on only a few noise stars, which shrinks their actual footprint inside the reference and forces the safe crop to shrink to the common overlap. Translation drift is the dominant cause here. Removing these frames would yield a larger crop.",
					cropRect.Dx(), cropRect.Dy(), cropPct, refW, refH, strings.Join(limiterNames, ", "))
			case anyFieldRotation:
				cropExplanation = fmt.Sprintf(
					"Crop rectangle is %dx%d (%.1f%% of the %dx%d reference). The safe crop is limited by frames whose field rotation pushes their footprint outside the reference: %s. This is normal for Alt-Az mounts or long unguided runs. Removing them would yield a larger crop but fewer stacked frames.",
					cropRect.Dx(), cropRect.Dy(), cropPct, refW, refH, strings.Join(limiterNames, ", "))
			default:
				cropExplanation = fmt.Sprintf(
					"Crop rectangle is %dx%d (%.1f%% of the %dx%d reference). The following frames constrain the crop or have low registration quality: %s. A normal frame has scale~1.0 and rotation~0deg (translation-only alignment); frames with scale far from 1 or rotation far from 0 were typically fitted on only a few noise stars, which shrinks their actual footprint inside the reference and forces the safe crop to shrink to the common overlap. Removing these frames would yield a larger crop.",
					cropRect.Dx(), cropRect.Dy(), cropPct, refW, refH, strings.Join(limiterNames, ", "))
			}
		}
		numAffine := 0
		for i := range transforms {
			if transforms[i].aff != nil {
				numAffine++
			}
		}
		if numAffine > 0 {
			cropExplanation += fmt.Sprintf(" %d frame(s) were refined with a full affine model (handles field rotation and mild optical distortion); the crop accounts for their true footprint.", numAffine)
		}
	}

	// Build validFrameWeights: filter frameWeights to only the frames
	// whose transforms are valid (those are the frames actually present
	// in spillPaths2, in the same order).
	var validFrameWeights []float32
	if registrationOn && len(spillPaths2) > 0 && len(frameWeights) > 0 {
		validFrameWeights = make([]float32, 0, len(spillPaths2))
		for i := range accepted {
			if transforms[i].valid {
				validFrameWeights = append(validFrameWeights, frameWeights[i])
			}
		}
	}

	// ============== Tiled median/sigma reduction (spill-based)
	var unregSumPix, unregMeanPix []float32
	var unregRejPix, unregWgtPix []float32
	if opts.Method == "median" || opts.Method == "sigma" {
		// Unregistered stack: full frame, all accepted frames.
		tr := opts.SigmaClipThreshold
		if tr <= 0 {
			tr = 3.0
		}
		it := opts.SigmaClipIters
		if it < 1 {
			it = 1
		}
		logf("Reducing unregistered stack (%s, %d frames, full frame)...",
			opts.Method, len(spillPaths1))
		sum1, mean1, rej1, wgt1, rerr := reduceTilesStack(
			spillPaths1, refW, refH, image.Rect(0, 0, refW, refH),
			opts.Method, tr, it, opts.TileSize, workers, unregWeights)
		if rerr != nil {
			return fmt.Errorf("reduce unregistered: %w", rerr)
		}
		unregSumPix = sum1
		unregMeanPix = mean1
		unregRejPix = rej1
		unregWgtPix = wgt1
		// Do NOT overwrite unregAcc. It holds the plain sum
		// accumulated in Pass 1 and is divided later to produce
		// stacked_no_correction_mean.jpg, which is always the
		// plain arithmetic mean, regardless of method.
		if len(spillPaths2) == 0 {
			// No registration: report reduction and write maps directly.
			writeGrayscaleMaps(outputDir, refW, refH, rej1, wgt1, "")
			logf("Reduction complete (unregistered only)")
		}

		// Registered stack (only if registration ran and produced spills).
		if registrationOn && len(spillPaths2) > 0 {
			logf("Reducing registered stack (%s, %d frames, crop %dx%d)...",
				opts.Method, len(spillPaths2), cropRect.Dx(), cropRect.Dy())
			sum2, mean2, rej2, wgt2, rerr := reduceTilesStack(
				spillPaths2, refW, refH, cropRect,
				opts.Method, tr, it, opts.TileSize, workers, validFrameWeights)
			if rerr != nil {
				return fmt.Errorf("reduce registered: %w", rerr)
			}
			regSumPix = sum2
			regMeanPix = mean2
			regRejPix = rej2
			regWgtPix = wgt2
			runtime.GC()
			logf("Reduction complete")
		}
	}

	// ============== Write outputs
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	workDir := filepath.Join(outputDir, "_work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("create work directory: %w", err)
	}
	tiffDir := filepath.Join(outputDir, "TIFF")
	if opts.CreateTIFF {
		if err := os.MkdirAll(tiffDir, 0o755); err != nil {
			return fmt.Errorf("create TIFF directory: %w", err)
		}
	}

	sumTiff := filepath.Join(tiffDir, "stacked_sum.tif")
	sumJPG := filepath.Join(outputDir, "stacked_sum.jpg")
	meanWorkPath := filepath.Join(workDir, "stacked_mean.f32raw")
	meanJPG := filepath.Join(outputDir, "stacked_mean.jpg")
	noCorrJPG := filepath.Join(outputDir, "stacked_no_correction_mean.jpg")
	reportTxt := filepath.Join(outputDir, "report.txt")
	maxJPG := filepath.Join(outputDir, "stacked_max.jpg")
	minJPG := filepath.Join(outputDir, "stacked_min.jpg")

	var sumMax float32
	var outW, outH int

	if registrationOn {
		outW, outH = cropRect.Dx(), cropRect.Dy()
		var sumPix, meanPix []float32
		if regSumPix != nil {
			// Tiled median/sigma reduction path.
			sumPix = regSumPix
			meanPix = regMeanPix
			sumMax = 0
			for _, v := range sumPix {
				if v > sumMax {
					sumMax = v
				}
			}
			if math.IsNaN(float64(sumMax)) || math.IsInf(float64(sumMax), 0) {
				sumMax = 1.0
			}
			if sumMax <= 0 {
				sumMax = 1.0
			}
		} else {
			// Streaming mean path.
			croppedSum := cropAccumulator(regAcc, cropRect)
			croppedCounts := cropCounts(validCount, refW, cropRect)
			sumMax = croppedSum.maxValue()
			if math.IsNaN(float64(sumMax)) || math.IsInf(float64(sumMax), 0) {
				sumMax = 1.0
			}
			sumPix = croppedSum.Pix
			croppedSum.divideByCount(croppedCounts)
			meanPix = croppedSum.Pix
		}
		sumDivisor := float32(1.0)
		if sumMax > 1.0 {
			sumDivisor = sumMax
		}
		logf("Registered sum max = %g (normalising sum TIFF by %g)", sumMax, sumDivisor)

		if opts.CreateTIFF {
			logf("Writing registered sum TIFF")
			if err := writeFileAtomic(sumTiff, func(w io.Writer) error {
				return writeFloat32RGBScaled(w, outW, outH, sumPix, sumDivisor)
			}); err != nil {
				return fmt.Errorf("write sum tiff: %w", err)
			}
		}
		logf("Writing registered sum preview")
		if err := writeFileAtomic(sumJPG, func(w io.Writer) error {
			return writeAutoNormalizedPreviewJPEG(w, outW, outH, sumPix)
		}); err != nil {
			return fmt.Errorf("write sum jpg: %w", err)
		}

		logf("Writing work file (stacked result, float32 raw)")
		if err := writeFileAtomic(meanWorkPath, func(w io.Writer) error {
			return writeRawFloat32RGB(w, outW, outH, meanPix)
		}); err != nil {
			return fmt.Errorf("write mean work file: %w", err)
		}
		logf("Writing registered mean preview")
		if err := writeFileAtomic(meanJPG, func(w io.Writer) error {
			return writePreviewJPEG(w, outW, outH, meanPix)
		}); err != nil {
			return fmt.Errorf("write mean jpg: %w", err)
		}
		if regRejPix != nil || regWgtPix != nil {
			writeGrayscaleMaps(outputDir, outW, outH, regRejPix, regWgtPix, "")
		}
		if regRejPix != nil {
			writeGrayscaleMaps(outputDir, refW, refH, unregRejPix, unregWgtPix, "_unregistered")
		}
		if validCount != nil {
			writeCoverageMap(outputDir, refW, refH, validCount, registeredCount, "")
		}
	} else {
		outW, outH = refW, refH
		var sumPix, meanPix []float32
		if unregSumPix != nil {
			sumPix = unregSumPix
			meanPix = unregMeanPix
			sumMax = 0
			for _, v := range sumPix {
				if v > sumMax {
					sumMax = v
				}
			}
			if math.IsNaN(float64(sumMax)) || math.IsInf(float64(sumMax), 0) || sumMax <= 0 {
				sumMax = 1.0
			}
		} else {
			sumMax = unregAcc.maxValue()
			if math.IsNaN(float64(sumMax)) || math.IsInf(float64(sumMax), 0) {
				sumMax = 1.0
			}
			sumPix = unregAcc.Pix
			unregAcc.divide()
			meanPix = unregAcc.Pix
		}
		sumDivisor := float32(1.0)
		if sumMax > 1.0 {
			sumDivisor = sumMax
		}
		logf("Sum max = %g (normalising sum TIFF by %g)", sumMax, sumDivisor)

		if opts.CreateTIFF {
			logf("Writing sum TIFF")
			if err := writeFileAtomic(sumTiff, func(w io.Writer) error {
				return writeFloat32RGBScaled(w, outW, outH, sumPix, sumDivisor)
			}); err != nil {
				return fmt.Errorf("write sum tiff: %w", err)
			}
		}
		logf("Writing sum preview")
		if err := writeFileAtomic(sumJPG, func(w io.Writer) error {
			return writeAutoNormalizedPreviewJPEG(w, outW, outH, sumPix)
		}); err != nil {
			return fmt.Errorf("write sum jpg: %w", err)
		}

		logf("Writing work file (stacked result, float32 raw)")
		if err := writeFileAtomic(meanWorkPath, func(w io.Writer) error {
			return writeRawFloat32RGB(w, outW, outH, meanPix)
		}); err != nil {
			return fmt.Errorf("write mean work file: %w", err)
		}
		logf("Writing mean preview")
		if err := writeFileAtomic(meanJPG, func(w io.Writer) error {
			return writePreviewJPEG(w, outW, outH, meanPix)
		}); err != nil {
			return fmt.Errorf("write mean jpg: %w", err)
		}
	}

	logf("Writing max (brightest pixels) preview")
	if err := writeFileAtomic(maxJPG, func(w io.Writer) error {
		return writePreviewJPEG(w, refW, refH, maxPix)
	}); err != nil {
		return fmt.Errorf("write max jpg: %w", err)
	}
	logf("Writing min (darkest pixels) preview")
	if err := writeFileAtomic(minJPG, func(w io.Writer) error {
		return writePreviewJPEG(w, refW, refH, minPix)
	}); err != nil {
		return fmt.Errorf("write min jpg: %w", err)
	}

	// Divide the unregistered accumulator regardless of mode.
	// divide() is idempotent: it is a no-op when Count == 0,
	// so calling it twice (once inside the mean path and
	// once here) is safe. With the fix in Pass 1 it is the
	// plain arithmetic mean over all accepted frames, which
	// is what stacked_no_correction_mean.jpg is documented to
	// be even when --method is median or sigma.
	unregAcc.divide()
	logf("Writing no-correction mean preview")
	if err := writeFileAtomic(noCorrJPG, func(w io.Writer) error {
		return writePreviewJPEG(w, refW, refH, unregAcc.Pix)
	}); err != nil {
		return fmt.Errorf("write no_correction_mean jpg: %w", err)
	}

	// Post-processing pipeline
	var postOutputs []string
	if opts.PostProcess && !opts.StackOnly {
		logf("Post-processing pipeline: bg-degree=%d stretch=%g colorCalib=%v sharpen=%v",
			opts.BGDegree, opts.StretchFactor, !opts.NoColorCalib, !opts.NoSharpen)
		outs, err := runPostProcessing(outputDir, opts)
		if err != nil {
			logf("WARNING: post-processing failed: %v", err)
		} else {
			postOutputs = outs
			logf("Post-processing complete: %d files", len(outs))
		}
	} else if opts.StackOnly {
		logf("Skipping post-processing (--stack-only)")
	}

	// Report
	notes := []string{
		"No calibration (dark, flat, bias) was performed.",
		"JPEG inputs are gamma-encoded (sRGB-like); samples were averaged in encoded space, not linear light.",
		"All JPG outputs are display previews, not scientific outputs.",
	}
	if regRejPix != nil || unregRejPix != nil {
		notes = append(notes,
			"stacked_rejection_map.jpg (and stacked_unregistered_rejection_map.jpg when "+
				"registration succeeded) count per-pixel sigma-clipped outliers. "+
				"Bright spots here are the cosmic-ray / hot-pixel candidates: sigma-clipping "+
				"rejects exactly those extreme outliers. "+
				"stacked_weight_map.jpg / stacked_unregistered_weight_map.jpg show the "+
				"surviving-frame count used by the sigma-clipped mean. "+
				"stacked_coverage_map.jpg shows the fraction of registered frames with a "+
				"valid sample at each pixel (full frame; every pixel inside the safe crop "+
				"is 100% by construction).",
		)
	}
	if registrationOn {
		notes = append(notes,
			fmt.Sprintf("Registration: %d of %d frames registered using a similarity model (rotation + uniform scale + translation).", registeredCount, len(accepted)),
			fmt.Sprintf("Registered outputs (stacked_sum/mean) are cropped to the largest fully-valid region: %dx%d.", outW, outH),
			"stacked_no_correction_mean is the unregistered full-frame mean.",
			fmt.Sprintf("Registered sum TIFF was divided by %g (global max of registered sum); multiply by %g to recover raw float32 sum.", sumMax, sumMax),
		)
		if len(regFailures) > 0 {
			notes = append(notes, fmt.Sprintf("%d frame(s) failed registration and were excluded from the registered stack; see 'Registration failures' above.", len(regFailures)))
		}
		if needsNormalize {
			if opts.LocalNormalize {
				notes = append(notes, fmt.Sprintf("Local normalization: per-tile scale+offset over %dx%d tiles of %d px, reference = %s.", normNTX, normNTY, opts.LNTileSize, filepath.Base(accepted[refIdx].path)))
			} else {
				notes = append(notes, fmt.Sprintf("Global normalization: per-frame scale+offset, reference = %s.", filepath.Base(accepted[refIdx].path)))
			}
		}
	} else {
		notes = append(notes,
			"Registration: disabled. Frames were combined in their existing pixel coordinate system.",
		)
		if opts.Method == "mean" {
			notes = append(notes, "stacked_mean.tiff and stacked_no_correction_mean.tiff are identical in this run (both are the plain arithmetic mean).")
		} else {
			notes = append(notes, fmt.Sprintf("stacked_mean.tiff is the %s of all frames; stacked_no_correction_mean.tiff is the plain arithmetic mean.", opts.Method))
		}
		notes = append(notes, fmt.Sprintf("Sum TIFF was divided by %g (global max); multiply by %g to recover raw float32 sum.", sumMax, sumMax))
	}

	acceptedPaths := make([]string, len(accepted))
	for i, c := range accepted {
		acceptedPaths[i] = c.path
	}

	rep := &processingReport{
		Version:           version,
		Timestamp:         start,
		InputDir:          inputDir,
		OutputDir:         outputDir,
		Discovered:        len(files),
		Accepted:          acceptedPaths,
		Rejected:          rejected,
		Width:             refW,
		Height:            refH,
		Channels:          3,
		Method:            methodString(registrationOn, opts.Method),
		AccumulatorType:   "float32 (interleaved RGB)",
		OutputFormat:      "TIFF: uncompressed, little-endian, RGB, IEEE float32, single strip. JPG: 8-bit, gamma 0.5 display preview.",
		OutputScaling:     scalingString(registrationOn, opts.Method),
		RegistrationOn:    registrationOn,
		RegisteredCount:   registeredCount,
		RegistrationFails: regFailures,
		CropRect:          cropRect,
		SumNormalization:  sumMax,
		OutputFiles: append(append(append([]string{},
			appendIf(opts.CreateTIFF, sumTiff)...),
			sumJPG, meanJPG, noCorrJPG, maxJPG, minJPG,
		), append(postOutputs, reportTxt)...),
		Workers:            workers,
		Duration:           time.Since(start),
		PostProcessApplied: opts.PostProcess && !opts.StackOnly,
		BGDegree:           opts.BGDegree,
		StretchFactor:      opts.StretchFactor,
		SharpenApplied:     opts.PostProcess && !opts.StackOnly && !opts.NoSharpen,
		SharpenAmount:      opts.SharpenAmount,
		SharpenRadius:      opts.SharpenRadius,
		Notes:              notes,
		FrameDiags:         frameDiags,
		CropExplanation:    cropExplanation,
		TotalExposureSec:   totalExposureSec,
		FramesWithExif:     framesWithExif,
		RefExif:            refExif,
	}
	if err := writeFileAtomic(reportTxt, func(w io.Writer) error {
		return rep.writeTo(w)
	}); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	logf("Actual processing time: %s", rep.Duration.Round(time.Millisecond))
	logf("Done -> %s", outputDir)
	return nil
}

func appendIf(cond bool, item string) []string {
	if cond {
		return []string{item}
	}
	return nil
}
func methodString(registered bool, method string) string {
	var prefix string
	switch method {
	case "median":
		prefix = "sum and per-pixel median"
	case "sigma":
		prefix = "sum and MAD-based sigma-clipped mean"
	default:
		prefix = "sum and arithmetic mean"
	}
	if registered {
		return prefix + " over registered frames; no_correction is the plain arithmetic mean over all frames"
	}
	return prefix + " (no registration); no_correction is the plain arithmetic mean"
}

func scalingString(registered bool, method string) string {
	var meanKind string
	switch method {
	case "median":
		meanKind = "per-pixel median"
	case "sigma":
		meanKind = "MAD-based sigma-clipped mean"
	default:
		meanKind = "arithmetic mean divided by per-pixel valid count"
	}
	base := "sum.tiff divided by its global max (recoverable via reported factor); stacked_mean is the " + meanKind + "; no_correction_mean.tiff is the plain arithmetic mean of all frames. Preview JPGs use gamma 0.5."
	if registered {
		return "registered " + base
	}
	return base
}

// =========================================================================
// Self-tests
// =========================================================================

type testCase struct {
	name string
	fn   func() error
}

func runSelfTests() int {
	cases := []testCase{
		{"accumulator sum and mean", testAccumulatorSumMean},
		{"accumulator overflow", testAccumulatorOverflow},
		{"accumulator dimension mismatch", testAccumulatorDimensionMismatch},
		{"accumulator parallel vs sequential", testAccumulatorParallel},
		{"accumulator max", testAccumulatorMax},
		{"accumulator masked", testAccumulatorMasked},
		{"TIFF round trip", testTIFFRoundTrip},
		{"TIFF large values", testTIFFLargeValues},
		{"TIFF scaled round trip", testTIFFScaledRoundTrip},
		{"TIFF bad input", testTIFFBadInput},
		{"decode RGBA", testDecodeRGBA},
		{"decode JPEG", testDecodeJPEG},
		{"discovery", testDiscovery},
		{"similarity estimate", testSimilarityEstimate},
		{"similarity inverse", testSimilarityInverse},
		{"star detection synthetic", testStarDetectionSynthetic},
		{"star registration synthetic", testStarRegistrationSynthetic},
		{"bilinear sampling", testBilinearSample},
		{"resample identity", testResampleIdentity},
		{"safe crop", testSafeCrop},
		{"postprocess polynomial basis", testPolyBasis},
		{"postprocess linear solve", testSolveLinear},
		{"postprocess background removal", testBackgroundRemoval},
		{"postprocess contrast stretch", testContrastStretch},
		{"postprocess color calibration", testColorCalibration},
		{"postprocess gaussian blur", testGaussianBlur},
		{"postprocess unsharp mask", testUnsharpMask},
		{"postprocess hdr", testHDR},
		{"postprocess reduce stars", testReduceStars},
		{"resample translation", testResampleTranslation},
		{"resample round trip", testResampleRoundTrip},
		{"registration with outliers", testRegistrationWithOutliers},
		{"star detection empty", testStarDetectionEmpty},
		{"star detection edge stars", testStarDetectionEdgeStars},
		{"star detection on bright background", testStarDetectionOnBrightBackground},
		{"parse args", testParseArgs},
		{"atomic write cleanup", testAtomicWriteCleanup},
		{"pipeline stack-only", testPipelineStackOnly},
		{"pipeline post-only", testPipelinePostOnly},
		{"remove stars basic", testRemoveStarsBasic},
		{"remove stars empty", testRemoveStarsEmpty},
		{"remove stars edge", testRemoveStarsEdge},
		{"remove stars radius", testRemoveStarsRadius},
		{"remove stars preserves background", testRemoveStarsPreservesBackground},
		{"detect stars sigma", testDetectStarsSigma},
		{"preview jpg removed", testPreviewJPGRemoved},
		{"output files list", testOutputFilesList},
		{"resize frame", testResizeFrame},
		{"tiff flag only sum", testTiffFlagOnlySum},
		{"star struct has size", testStarStructHasSize},
		{"detect stars sigma single pixel", testDetectStarsSigmaSinglePixel},
		{"remove stars fine stars", testRemoveStarsFineStars},
		{"find CR2 tool", testFindCR2Tool},
		{"parse dcraw output", testParseDcrawOutput},
		{"black point shift", testBlackPointShift},
		{"MTF", testMTF},
		{"auto stretch", testAutoStretch},
		{"strict uniform dimensions", testStrictUniformDimensions},
		{"same dimensions succeed", testSameDimensionsSucceeds},
		{"guided filter gray", testGuidedFilterGray},
		{"denoise frame", testDenoiseFrame},
		{"denoise iterations", testDenoiseFrameIterations},
		{"blend weighted", testBlendWeighted},
		{"HDR blend", testHDRBlend},
		{"compute frame quality", testComputeFrameQuality},
		{"frame box in reference", testFrameBoxInReference},
		{"match rejects few inliers", testMatchStarsRejectsFewInliers},
		{"median in place", testMedianInPlace},
		{"mad of", testMadOf},
		{"sigma clip reduce", testSigmaClipReduce},
		{"sigma clip all same", testSigmaClipAllSame},
		{"sigma clip reduce weighted", testSigmaClipReduceWeighted},
		{"read tile from raw", testReadTileFromRaw},
		{"reduce tiles stack weighted", testReduceTilesStackWeighted},
		{"reduce tiles stack", testReduceTilesStack},
		{"chroma denoise", testChromaDenoiseGB},
		{"chroma denoise multi-scale", testChromaDenoiseMultiScale},
		{"apply curves", testApplyCurves},
		{"add diffraction spikes", testAddDiffractionSpikes},
		{"spikes min count", testSpikesMinCount},
		{"estimate affine", testEstimateAffine},
		{"affine inverse round trip", testAffineInverseRoundTrip},
		{"high pass radius adaptive", testHighPassRadiusAdaptive},
		{"disk space guard", testDiskSpaceGuard},
		{"intrinsic quality zero stars", testIntrinsicQualityZeroStars},
		{"compute norm stats", testComputeNormStats},
		{"global normalize offset", testGlobalNormalizeOffset},
		{"global normalize identity", testGlobalNormalizeIdentity},
		{"local normalize gradient", testLocalNormalizeGradient},
		{"local normalize masked pixels", testLocalNormalizeMaskedPixels},
		{"frame limiter flags", testFrameLimiterFlags},
		{"frame limiter flags rotation aware", testFrameLimiterFlagsRotationAware},
		{"star cache round trip", testStarCacheRoundTrip},
		{"star cache invalidation", testStarCacheInvalidation},
		{"xf cache invalidation", testXfCacheInvalidation},
		{"report contains", testReportContains},
		{"report shows weight columns", testReportShowsWeightColumns},
		{"write grayscale maps suffix", testWriteGrayscaleMapsSuffix},
		{"write coverage map", testWriteCoverageMap},
		{"detect calibration subdirs", testDetectCalibrationSubdirs},
		{"fits native read", testFITSNative},
		{"fits debayer", testFITSDebayer},
		{"fits debayer CLI", testDebayerCLI},
	}
	failures := 0
	for _, c := range cases {
		if err := c.fn(); err != nil {
			fmt.Printf("FAIL %s: %v\n", c.name, err)
			failures++
		} else {
			fmt.Printf("ok   %s\n", c.name)
		}
	}
	if failures > 0 {
		fmt.Printf("\n%d of %d self-tests failed\n", failures, len(cases))
	} else {
		fmt.Printf("\nall %d self-tests passed\n", len(cases))
	}
	return failures
}

func testAccumulatorSumMean() error {
	a := newAccumulator(2, 2)
	f1 := &frame{Width: 2, Height: 2, Pix: []float32{
		0.1, 0.2, 0.3, 0.4, 0.5, 0.6,
		0.7, 0.8, 0.9, 1.0, 0.0, 0.5,
	}}
	f2 := &frame{Width: 2, Height: 2, Pix: []float32{
		0.9, 0.8, 0.7, 0.6, 0.5, 0.4,
		0.3, 0.2, 0.1, 0.0, 1.0, 0.5,
	}}
	if err := a.addFrame(f1, 1); err != nil {
		return err
	}
	if err := a.addFrame(f2, 1); err != nil {
		return err
	}
	for i, v := range a.Pix {
		if math.Abs(float64(v-1.0)) > 1e-6 {
			return fmt.Errorf("sum[%d] = %v, want 1", i, v)
		}
	}
	a.divide()
	for i, v := range a.Pix {
		if math.Abs(float64(v-0.5)) > 1e-6 {
			return fmt.Errorf("mean[%d] = %v, want 0.5", i, v)
		}
	}
	return nil
}

func testAccumulatorOverflow() error {
	a := newAccumulator(1, 1)
	f := &frame{Width: 1, Height: 1, Pix: []float32{1, 1, 1}}
	for i := 0; i < 300; i++ {
		if err := a.addFrame(f, 1); err != nil {
			return err
		}
	}
	if a.Pix[0] != 300 {
		return fmt.Errorf("sum = %v, want 300", a.Pix[0])
	}
	a.divide()
	if math.Abs(float64(a.Pix[0]-1)) > 1e-6 {
		return fmt.Errorf("mean = %v, want 1", a.Pix[0])
	}
	return nil
}

func testAccumulatorDimensionMismatch() error {
	a := newAccumulator(2, 2)
	bad := &frame{Width: 3, Height: 2, Pix: make([]float32, 3*2*3)}
	if err := a.addFrame(bad, 1); err == nil {
		return errors.New("expected dimension-mismatch error")
	}
	return nil
}

func testAccumulatorParallel() error {
	a := newAccumulator(64, 32)
	f := &frame{Width: 64, Height: 32, Pix: make([]float32, 64*32*3)}
	for i := range f.Pix {
		f.Pix[i] = float32(i%97) / 97.0
	}
	if err := a.addFrame(f, 8); err != nil {
		return err
	}
	b := newAccumulator(64, 32)
	if err := b.addFrame(f, 1); err != nil {
		return err
	}
	for i := range a.Pix {
		if a.Pix[i] != b.Pix[i] {
			return fmt.Errorf("mismatch at %d", i)
		}
	}
	return nil
}

func testAccumulatorMax() error {
	a := newAccumulator(2, 1)
	a.Pix = []float32{0.1, 0.5, 3.2, 1.1, 0.9, 2.7}
	if got := a.maxValue(); got != 3.2 {
		return fmt.Errorf("max = %v, want 3.2", got)
	}
	return nil
}

func testAccumulatorMasked() error {
	a := newAccumulator(2, 1)
	valid := make([]int32, 2)
	f1 := &frame{Width: 2, Height: 1, Pix: []float32{0.5, 0.6, 0.7, -1, -1, -1}}
	f2 := &frame{Width: 2, Height: 1, Pix: []float32{-1, -1, -1, 0.9, 0.8, 0.7}}
	if err := a.addFrameMasked(f1, 1, valid); err != nil {
		return err
	}
	if err := a.addFrameMasked(f2, 1, valid); err != nil {
		return err
	}
	if valid[0] != 1 || valid[1] != 1 {
		return fmt.Errorf("validCounts = %v, want [1 1]", valid)
	}
	a.divideByCount(valid)
	for i, v := range a.Pix {
		want := []float32{0.5, 0.6, 0.7, 0.9, 0.8, 0.7}[i]
		if math.Abs(float64(v-want)) > 1e-6 {
			return fmt.Errorf("masked[%d] = %v, want %v", i, v, want)
		}
	}
	return nil
}

func testTIFFRoundTrip() error {
	const w, h = 3, 2
	pix := []float32{
		0, 0.25, 0.5,
		1, 0.5, 0.75,
		0.125, 0.375, 0.625,
		0.1, 0.2, 0.3,
		0, 0, 1,
		0.99, 0.5, 0.01,
	}
	var buf bytes.Buffer
	if err := writeFloat32RGB(&buf, w, h, pix); err != nil {
		return err
	}
	gotW, gotH, gotPix, err := readFloat32RGB(&buf)
	if err != nil {
		return err
	}
	if gotW != w || gotH != h {
		return fmt.Errorf("dims")
	}
	for i := range pix {
		if gotPix[i] != pix[i] {
			return fmt.Errorf("pixel[%d]", i)
		}
	}
	return nil
}

func testTIFFLargeValues() error {
	var buf bytes.Buffer
	pix := []float32{1000000, 999999.5, 3.4e38}
	if err := writeFloat32RGB(&buf, 1, 1, pix); err != nil {
		return err
	}
	_, _, got, err := readFloat32RGB(&buf)
	if err != nil {
		return err
	}
	for i := range pix {
		if math.Abs(float64(got[i]-pix[i]))/math.Abs(float64(pix[i])) > 1e-6 {
			return fmt.Errorf("pixel[%d]", i)
		}
	}
	return nil
}

func testTIFFScaledRoundTrip() error {
	var buf bytes.Buffer
	pix := []float32{10, 20, 30, 40, 50, 60}
	if err := writeFloat32RGBScaled(&buf, 2, 1, pix, 10); err != nil {
		return err
	}
	_, _, got, err := readFloat32RGB(&buf)
	if err != nil {
		return err
	}
	want := []float32{1, 2, 3, 4, 5, 6}
	for i := range want {
		if math.Abs(float64(got[i]-want[i])) > 1e-6 {
			return fmt.Errorf("scaled[%d]", i)
		}
	}
	return nil
}

func testTIFFBadInput() error {
	if err := writeFloat32RGB(&bytes.Buffer{}, 0, 0, nil); err == nil {
		return errors.New("expected error for zero dimensions")
	}
	if err := writeFloat32RGB(&bytes.Buffer{}, 2, 2, make([]float32, 11)); err == nil {
		return errors.New("expected error for bad buffer")
	}
	if err := writeFloat32RGBScaled(&bytes.Buffer{}, 2, 2, make([]float32, 12), 0); err == nil {
		return errors.New("expected error for zero divisor")
	}
	return nil
}

func testDecodeRGBA() error {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	img.SetRGBA(1, 0, color.RGBA{0, 255, 0, 255})
	f, err := frameFromImage(img)
	if err != nil {
		return err
	}
	checks := []struct {
		x, y, c int
		want    float32
	}{
		{0, 0, 0, 1}, {0, 0, 1, 0}, {0, 0, 2, 0},
		{1, 0, 0, 0}, {1, 0, 1, 1}, {1, 0, 2, 0},
	}
	for _, ck := range checks {
		got := f.Pix[(ck.y*f.Width+ck.x)*3+ck.c]
		if math.Abs(float64(got-ck.want)) > 1e-6 {
			return fmt.Errorf("pixel")
		}
	}
	return nil
}

func testDecodeJPEG() error {
	src := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			src.SetRGBA(x, y, color.RGBA{200, 100, 50, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 95}); err != nil {
		return err
	}
	img, _, err := image.Decode(&buf)
	if err != nil {
		return err
	}
	f, err := frameFromImage(img)
	if err != nil {
		return err
	}
	want := [3]float32{200.0 / 255.0, 100.0 / 255.0, 50.0 / 255.0}
	for i := 0; i < 4*3; i++ {
		for c := 0; c < 3; c++ {
			if math.Abs(float64(f.Pix[i*3+c]-want[c])) > 0.02 {
				return fmt.Errorf("pixel %d ch %d = %v, want ~%v", i, c, f.Pix[i*3+c], want[c])
			}
		}
	}
	return nil
}

func testDiscovery() error {
	dir, err := os.MkdirTemp("", "skystacker-discovery-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	must := func(p string) error {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		return os.WriteFile(p, []byte("x"), 0o644)
	}
	for _, p := range []string{
		filepath.Join(dir, "b.JPG"),
		filepath.Join(dir, "a.jpeg"),
		filepath.Join(dir, "c.tif"),
		filepath.Join(dir, "sub", "d.jpg"),
	} {
		if err := must(p); err != nil {
			return err
		}
	}
	got, err := discoverFiles(dir, false, nil)
	if err != nil {
		return err
	}
	want := []string{filepath.Join(dir, "a.jpeg"), filepath.Join(dir, "b.JPG")}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		return fmt.Errorf("discovery: got %v, want %v", got, want)
	}

	// PNG is now a first-class input (see discoverFiles); a .png file
	// must be discovered alongside JPGs, while .tif remains excluded.
	if err := must(filepath.Join(dir, "starfield.png")); err != nil {
		return err
	}
	got2, err := discoverFiles(dir, false, nil)
	if err != nil {
		return err
	}
	found := false
	for _, p := range got2 {
		if filepath.Base(p) == "starfield.png" {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("PNG file not discovered: %v", got2)
	}
	return nil
}

func testSimilarityEstimate() error {
	// 3 point pairs under known similarity
	src := []point2D{{0, 0}, {10, 0}, {0, 10}}
	dst := []point2D{{5, 3}, {15, 3}, {5, 13}} // pure translation (5, 3)
	sim := estimateSimilarity(src, dst)
	if math.Abs(sim.SC-1) > 1e-9 || math.Abs(sim.SS) > 1e-9 {
		return fmt.Errorf("SC=%v SS=%v, want 1, 0", sim.SC, sim.SS)
	}
	if math.Abs(sim.Tx-5) > 1e-9 || math.Abs(sim.Ty-3) > 1e-9 {
		return fmt.Errorf("T=(%v,%v), want (5,3)", sim.Tx, sim.Ty)
	}
	return nil
}

func testSimilarityInverse() error {
	// Rotation 90°, translation (1, 2), unit scale
	sim := similarity{SC: 0, SS: 1, Tx: 1, Ty: 2}
	inv := sim.inverse()
	p := point2D{3, 4}
	q := sim.apply(p)
	back := inv.apply(q)
	if math.Abs(back.X-p.X) > 1e-9 || math.Abs(back.Y-p.Y) > 1e-9 {
		return fmt.Errorf("inverse roundtrip: %v -> %v -> %v", p, q, back)
	}
	return nil
}

func testStarDetectionSynthetic() error {
	// Create a 100×100 frame with 5 Gaussian-like blobs.
	const w, h = 100, 100
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.05
	}
	stars := []point2D{{20, 20}, {50, 50}, {80, 30}, {30, 80}, {70, 70}}
	for _, s := range stars {
		for dy := -3; dy <= 3; dy++ {
			for dx := -3; dx <= 3; dx++ {
				x, y := int(s.X)+dx, int(s.Y)+dy
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				d2 := float64(dx*dx + dy*dy)
				v := float32(0.9 * math.Exp(-d2/4))
				i := (y*w + x) * 3
				f.Pix[i+0] += v
				f.Pix[i+1] += v
				f.Pix[i+2] += v
			}
		}
	}
	detected := detectStars(f, 20)
	if len(detected) < 5 {
		return fmt.Errorf("detected %d stars, expected at least 5", len(detected))
	}
	// Each detected star should be close to one of the 5 known positions.
	for _, ds := range detected[:5] {
		best := 1e9
		for _, s := range stars {
			d := math.Hypot(ds.X-s.X, ds.Y-s.Y)
			if d < best {
				best = d
			}
		}
		if best > 2.0 {
			return fmt.Errorf("detected star at (%v,%v) is %v px from nearest true star", ds.X, ds.Y, best)
		}
	}
	return nil
}

func testStarRegistrationSynthetic() error {
	// Reference stars
	refStars := []star{
		{X: 100, Y: 100, Flux: 1000}, {X: 200, Y: 150, Flux: 800}, {X: 300, Y: 200, Flux: 600},
		{X: 150, Y: 300, Flux: 700}, {X: 250, Y: 350, Flux: 500}, {X: 400, Y: 250, Flux: 400},
		{X: 180, Y: 220, Flux: 300}, {X: 280, Y: 180, Flux: 250}, {X: 350, Y: 300, Flux: 200},
	}
	// Known T: rotation 2°, translation (5, 3), unit scale
	theta := 2.0 * math.Pi / 180
	T := similarity{
		SC: math.Cos(theta),
		SS: math.Sin(theta),
		Tx: 5,
		Ty: 3,
	}
	Tinv := T.inverse()
	var tgtStars []star
	for _, s := range refStars {
		p := Tinv.apply(point2D{s.X, s.Y})
		tgtStars = append(tgtStars, star{X: p.X, Y: p.Y, Flux: s.Flux})
	}
	recovered, inliers, ok := matchStarsToReference(refStars, tgtStars)
	if !ok {
		return errors.New("matchStarsToReference returned ok=false")
	}
	if inliers < len(refStars)-1 {
		return fmt.Errorf("inliers = %d, want ~%d", inliers, len(refStars))
	}
	for i := range tgtStars {
		expected := point2D{refStars[i].X, refStars[i].Y}
		actual := recovered.apply(point2D{tgtStars[i].X, tgtStars[i].Y})
		if math.Hypot(actual.X-expected.X, actual.Y-expected.Y) > 1.0 {
			return fmt.Errorf("star %d: recovered (%v) != expected (%v)", i, actual, expected)
		}
	}
	return nil
}

func testBilinearSample() error {
	f := &frame{Width: 2, Height: 2, Pix: []float32{
		0, 0, 0, 1, 0, 0, // (0,0)=(0,0,0), (1,0)=(1,0,0)
		0, 1, 0, 1, 1, 0, // (0,1)=(0,1,0), (1,1)=(1,1,0)
	}}
	// At (0.5, 0.5) bilinear = average of 4 = (0.5, 0.5, 0)
	r, g, b, ok := sampleBilinear(f, 0.5, 0.5)
	if !ok {
		return errors.New("sampleBilinear(0.5, 0.5) returned ok=false")
	}
	if math.Abs(float64(r)-0.5) > 1e-6 || math.Abs(float64(g)-0.5) > 1e-6 || math.Abs(float64(b)) > 1e-6 {
		return fmt.Errorf("(0.5,0.5) = (%v,%v,%v), want (0.5,0.5,0)", r, g, b)
	}
	// Out of bounds
	if _, _, _, ok := sampleBilinear(f, -0.1, 0); ok {
		return errors.New("expected out-of-bounds")
	}
	return nil
}

func testResampleIdentity() error {
	f := &frame{Width: 5, Height: 5, Pix: make([]float32, 5*5*3)}
	for i := range f.Pix {
		f.Pix[i] = float32(i) / float32(len(f.Pix))
	}
	out := resampleToReference(f, identitySimilarity, 5, 5, 1)
	for i := range f.Pix {
		if math.Abs(float64(out.Pix[i]-f.Pix[i])) > 1e-5 {
			return fmt.Errorf("identity resample: pixel %d = %v, want %v", i, out.Pix[i], f.Pix[i])
		}
	}
	return nil
}

func testSafeCrop() error {
	const w, h = 10, 10
	valid := make([]int32, w*h)
	for i := range valid {
		valid[i] = 3 // all valid
	}
	// Carve out an invalid region: row 0
	for x := 0; x < w; x++ {
		valid[x] = 2
	}
	crop := findSafeCrop(valid, w, h, 3)
	if crop.Min.Y != 1 || crop.Max.Y != 10 {
		return fmt.Errorf("crop y = %d..%d, want 1..10", crop.Min.Y, crop.Max.Y)
	}
	if crop.Min.X != 0 || crop.Max.X != 10 {
		return fmt.Errorf("crop x = %d..%d, want 0..10", crop.Min.X, crop.Max.X)
	}
	return nil
}

// =========================================================================
// v0.4.3 TEST ADDITIONS — resampling, registration robustness, CLI, pipeline
// =========================================================================

// testResampleTranslation checks that a non-identity similarity actually
// moves samples. This is the single most important test missing from v0.4.2:
// every existing resampling test uses identitySimilarity, so a sign flip in
// sampleBilinear or a direction error in similarity would go unnoticed.
func testResampleTranslation() error {
	const w, h = 5, 5
	src := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	// Bright pixel at (1, 1)
	idx := (1*w + 1) * 3
	src.Pix[idx], src.Pix[idx+1], src.Pix[idx+2] = 1, 1, 1

	// sim maps source → reference by translating (+2, 0).
	// So reference pixel (3, 1) samples source (1, 1) → should be bright.
	sim := similarity{SC: 1, SS: 0, Tx: 2, Ty: 0}
	out := resampleToReference(src, sim, w, h, 1)

	if got := out.Pix[(1*w+3)*3]; got < 0.9 {
		return fmt.Errorf("expected bright at (3,1), got %v", got)
	}
	if got := out.Pix[(1*w+1)*3]; got > 0.1 {
		return fmt.Errorf("expected dark at (1,1), got %v", got)
	}
	return nil
}

// testResampleRoundTrip: forward then inverse transform must recover the
// source for integer translations (bilinear at integer offsets is exact).
// This exercises similarity.inverse(), which was previously tested only
// for a single point pair, not through the whole resampler.
func testResampleRoundTrip() error {
	const w, h = 8, 8
	src := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range src.Pix {
		src.Pix[i] = float32((i*37)%100) / 100
	}

	sim := similarity{SC: 1, SS: 0, Tx: -2, Ty: 3}
	inv := sim.inverse()

	fwd := resampleToReference(src, sim, w, h, 1)
	// Resample fwd back through inv: net transform is identity, so pixels
	// that remained in bounds should match src.
	back := resampleToReference(fwd, inv, w, h, 1)

	diff := 0
	for y := 2; y < h-3; y++ {
		for x := 2; x < w-3; x++ {
			i := (y*w + x) * 3
			if math.Abs(float64(back.Pix[i]-src.Pix[i])) > 1e-4 {
				diff++
			}
		}
	}
	if diff > 0 {
		return fmt.Errorf("%d interior pixels differ after round-trip", diff)
	}
	return nil
}

// testRegistrationWithOutliers: RANSAC must survive 10 spurious "stars" in
// the target frame. Without this, a session with satellite trails or hot
// pixels could cause the transform to be fitted to garbage.
func testRegistrationWithOutliers() error {
	refStars := []star{
		{X: 100, Y: 100, Flux: 1000}, {X: 200, Y: 150, Flux: 800},
		{X: 300, Y: 200, Flux: 600}, {X: 150, Y: 300, Flux: 700},
		{X: 250, Y: 350, Flux: 500}, {X: 400, Y: 250, Flux: 400},
		{X: 180, Y: 220, Flux: 300}, {X: 280, Y: 180, Flux: 250},
		{X: 350, Y: 300, Flux: 200},
	}
	theta := 2.0 * math.Pi / 180
	T := similarity{SC: math.Cos(theta), SS: math.Sin(theta), Tx: 5, Ty: 3}
	Tinv := T.inverse()

	var tgtStars []star
	for _, s := range refStars {
		p := Tinv.apply(point2D{s.X, s.Y})
		tgtStars = append(tgtStars, star{X: p.X, Y: p.Y, Flux: s.Flux})
	}
	// 10 spurious detections scattered across the field
	for i := 0; i < 10; i++ {
		tgtStars = append(tgtStars, star{
			X: float64(500 + i*15), Y: float64(500 + i*11), Flux: 50,
		})
	}

	_, inliers, ok := matchStarsToReference(refStars, tgtStars)
	if !ok {
		return errors.New("matchStarsToReference failed with outliers present")
	}
	if inliers < 8 {
		return fmt.Errorf("only %d inliers, want ≥8 of 9 true stars", inliers)
	}
	return nil
}

// testStarDetectionEmpty: a perfectly uniform frame must return zero stars
// and must not panic. Current detectStars has a "no comps" fast path that
// is untested.
func testStarDetectionEmpty() error {
	const w, h = 64, 64
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.05
	}
	got := detectStars(f, 100)
	if len(got) != 0 {
		return fmt.Errorf("expected 0 stars, got %d", len(got))
	}
	return nil
}

// testStarDetectionEdgeStars: stars touching the margin filter must be
// rejected. Without this, registration can match on partially-clipped
// stars whose centroids are wrong by several pixels.
func testStarDetectionEdgeStars() error {
	const w, h = 40, 40
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.05
	}
	// Place stars at (1,1) — inside margin — and (20,20) — well inside
	place := func(cx, cy int) {
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				x, y := cx+dx, cy+dy
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				i := (y*w + x) * 3
				f.Pix[i] += 0.5
				f.Pix[i+1] += 0.5
				f.Pix[i+2] += 0.5
			}
		}
	}
	place(1, 1)
	place(20, 20)
	got := detectStars(f, 100)
	for _, s := range got {
		if s.X < 3 || s.Y < 3 {
			return fmt.Errorf("edge star not rejected: (%v, %v)", s.X, s.Y)
		}
	}
	return nil
}

// testParseArgs: invalid CLI configurations must be rejected at parse time.
// Right now any --bg-degree, including "0" or "99", is accepted and silently
// clamped inside removeBackground.
// testStarDetectionOnBrightBackground verifies that stars sitting on a
// bright nebula-like gradient are detected just like stars on dark sky.
// Before the high-pass fix, detectStarsSigma used a single global
// threshold, which either missed these stars or merged the whole nebula
// into one giant rejected component.
func testStarDetectionOnBrightBackground() error {
	const w, h = 200, 200
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	// Radial nebula: 0.5 in the centre, 0.05 at the edges.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x-100) / 100.0
			dy := float64(y-100) / 100.0
			d := math.Sqrt(dx*dx + dy*dy)
			if d > 1 {
				d = 1
			}
			v := float32(0.5 - 0.45*d)
			i := (y*w + x) * 3
			f.Pix[i] = v
			f.Pix[i+1] = v
			f.Pix[i+2] = v
		}
	}
	// A star at the very centre of the nebula (brightest local background).
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			i := ((100+dy)*w + (100 + dx)) * 3
			f.Pix[i] += 0.4
			f.Pix[i+1] += 0.4
			f.Pix[i+2] += 0.4
		}
	}
	// A second star on dark sky for comparison.
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			i := ((20+dy)*w + (20 + dx)) * 3
			f.Pix[i] += 0.4
			f.Pix[i+1] += 0.4
			f.Pix[i+2] += 0.4
		}
	}
	detected := detectStarsSigma(f, 100, 4.0)
	nebulaStar := false
	skyStar := false
	for _, s := range detected {
		if math.Abs(s.X-100) < 2 && math.Abs(s.Y-100) < 2 {
			nebulaStar = true
		}
		if math.Abs(s.X-20) < 2 && math.Abs(s.Y-20) < 2 {
			skyStar = true
		}
	}
	if !nebulaStar {
		return fmt.Errorf("star on bright nebula not detected (got %d stars)", len(detected))
	}
	if !skyStar {
		return fmt.Errorf("star on dark sky not detected (got %d stars)", len(detected))
	}
	return nil
}

func testParseArgs() error {
	cases := []struct {
		args    []string
		wantErr bool
	}{
		{[]string{"--version"}, false},
		{[]string{"--bg-degree", "5"}, true},
		{[]string{"--bg-degree", "0"}, true},
		{[]string{"--stretch", "-1"}, true},
		{[]string{"--workers", "0"}, true},
		{[]string{"--sharpen-amount", "-1"}, true},
		{[]string{"--sharpen-radius", "0"}, true},
		{[]string{"--stack-only", "--post-only"}, true},
		{[]string{"a", "b"}, true}, // two positional args
		{nil, false},
	}
	for i, c := range cases {
		_, err := parseArgs(c.args)
		if (err != nil) != c.wantErr {
			return fmt.Errorf("case %d (%v): wantErr=%v, got err=%v",
				i, c.args, c.wantErr, err)
		}
	}
	return nil
}

// testAtomicWriteCleanup: a failed write must not leave a .tmp file behind.
// The atomic write is the only safety net against partially-written outputs;
// a .tmp leak there would silently fill disks on every failed run.
func testAtomicWriteCleanup() error {
	dir, err := os.MkdirTemp("", "skystacker-atomic-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "out.bin")
	writeErr := writeFileAtomic(path, func(w io.Writer) error {
		_, _ = w.Write([]byte("partial"))
		return errors.New("simulated encoder failure")
	})
	if writeErr == nil {
		return errors.New("expected error from failing writer")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("output file exists after failure")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		return fmt.Errorf("orphan .tmp file after failure")
	}
	return nil
}

// testPipelineStackOnly: end-to-end through runPipeline with --stack-only.
// This is the highest-level test the project has and catches regressions
// in output naming, report writing, and directory creation.

func testHDR() error {
	f := &frame{Width: 10, Height: 10, Pix: make([]float32, 10*10*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.2
	}
	out := applyHDR(f, 0.5)
	for _, v := range out.Pix {
		if v < 0 || v > 1 {
			return errors.New("HDR out of bounds")
		}
	}
	return nil
}

func testReduceStars() error {
	f := &frame{Width: 5, Height: 5, Pix: make([]float32, 5*5*3)}
	// Create a fake star
	for i := range f.Pix {
		f.Pix[i] = 0.1
	}
	f.Pix[(2*5+2)*3] = 1.0
	f.Pix[(2*5+3)*3] = 1.0
	out := reduceStars(f, 1.0)
	if out.Pix[(2*5+2)*3] >= 1.0 {
		return errors.New("star was not reduced")
	}
	return nil
}

func testPipelineStackOnly() error {
	dir, err := os.MkdirTemp("", "skystacker-e2e-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	const w, h = 32, 32
	for i := 0; i < 3; i++ {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.SetRGBA(x, y, color.RGBA{100, 100, 100, 255})
			}
		}
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				img.SetRGBA(16+dx, 16+dy, color.RGBA{255, 255, 255, 255})
			}
		}
		path := filepath.Join(dir, fmt.Sprintf("frame_%02d.jpg", i))
		fp, err := os.Create(path)
		if err != nil {
			return err
		}
		if err := jpeg.Encode(fp, img, &jpeg.Options{Quality: 95}); err != nil {
			fp.Close()
			return err
		}
		fp.Close()
	}

	if err := runPipeline(context.Background(), runOptions{
		InputDir: dir, Workers: 2, Register: true, StackOnly: true, Scale: 1,
	}); err != nil {
		return fmt.Errorf("runPipeline: %w", err)
	}

	outDir := filepath.Join(dir, "0_skystacker_results")
	for _, name := range []string{
		"_work/stacked_mean.f32raw", "stacked_mean.jpg",
		"stacked_no_correction_mean.jpg",
		"stacked_sum.jpg", "stacked_max.jpg", "stacked_min.jpg", "report.txt",
	} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			return fmt.Errorf("missing output %s: %w", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "TIFF")); err == nil {
		return errors.New("TIFF folder should not exist without --tiff flag")
	}
	return nil
}

// testPipelinePostOnly: --post-only must consume an existing stacked_mean.tiff
// and produce the four step files. This path is completely untested today.
func testPipelinePostOnly() error {
	dir, err := os.MkdirTemp("", "skystacker-postonly-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	outDir := filepath.Join(dir, "0_skystacker_results")
	os.MkdirAll(outDir, 0o755)
	os.MkdirAll(filepath.Join(outDir, "_work"), 0o755)

	const w, h = 64, 64
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w*h; i++ {
		x, y := i%w, i/w
		bg := float32(0.1 + 0.3*float64(x)/float64(w) + 0.1*float64(y)/float64(h))
		f.Pix[i*3] = bg
		f.Pix[i*3+1] = bg
		f.Pix[i*3+2] = bg
	}
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			i := ((30+dy)*w + 32 + dx) * 3
			f.Pix[i] += 0.5
			f.Pix[i+1] += 0.5
			f.Pix[i+2] += 0.5
		}
	}
	if err := writeFileAtomic(filepath.Join(outDir, "_work", "stacked_mean.f32raw"),
		func(out io.Writer) error { return writeRawFloat32RGB(out, w, h, f.Pix) }); err != nil {
		return err
	}

	if err := runPipeline(context.Background(), runOptions{
		InputDir: dir, PostOnly: true, BGDegree: 2, StretchFactor: 10,
		SharpenAmount: 0.8, SharpenRadius: 1.5, BgPercentile: 10,
		HDRBlendFactor: 1.0, DenoiseStrength: 1.0, DenoiseRadius: 3,
	}); err != nil {
		return fmt.Errorf("runPipeline --post-only: %w", err)
	}

	for _, name := range []string{
		"post_step1_background_removed.jpg",
		"post_step2_star_removed.jpg",
		"post_step4_auto_stretched.jpg",
		"post_step5_denoised.jpg",
		"post_step6_curves.jpg",
		"post_step7_hdr.jpg",
		"post_step8_1.jpg",
		"post_step8_2.jpg",
		"post_step8_3.jpg",
		"post_step8_4.jpg",
	} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			return fmt.Errorf("missing post output %s: %w", name, err)
		}
	}
	for _, name := range []string{
		"post_step_ca.jpg", "post_step_color_calibrated.jpg",
		"post_step_flattened.jpg", "post_step_sharpened.jpg",
		"post_step_hdr.jpg", "post_step_stretch_gentle.jpg",
	} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err == nil {
			return fmt.Errorf("unexpected output %s (should require flag)", name)
		}
	}
	return nil
}

// testReportContains: report.txt must contain the exact caveats the project
// promises in its documentation. A silent drop of the "no registration"
// caveat would ship a scientifically misleading report.

func testRemoveStarsBasic() error {
	const w, h = 100, 100
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w*h; i++ {
		f.Pix[i*3] = 0.1
		f.Pix[i*3+1] = 0.1
		f.Pix[i*3+2] = 0.1
	}
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			i := ((50+dy)*w + (50 + dx)) * 3
			f.Pix[i] = 0.9
			f.Pix[i+1] = 0.9
			f.Pix[i+2] = 0.9
		}
	}
	result := removeStars(f, 4.0, 15)
	i := (50*w + 50) * 3
	if result.Pix[i] > 0.3 {
		return fmt.Errorf("star not removed: center=%v", result.Pix[i])
	}
	j := (10*w + 10) * 3
	if math.Abs(float64(result.Pix[j]-0.1)) > 0.05 {
		return fmt.Errorf("background altered: %v", result.Pix[j])
	}
	return nil
}

func testRemoveStarsEmpty() error {
	f := &frame{Width: 50, Height: 50, Pix: make([]float32, 50*50*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.1
	}
	result := removeStars(f, 4.0, 10)
	for i := range result.Pix {
		if math.Abs(float64(result.Pix[i]-0.1)) > 0.001 {
			return fmt.Errorf("uniform background altered at %d: %v", i, result.Pix[i])
		}
	}
	return nil
}

func testRemoveStarsEdge() error {
	const w, h = 50, 50
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.1
	}
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			x, y := 1+dx, 1+dy
			if x < 0 || y < 0 || x >= w || y >= h {
				continue
			}
			i := (y*w + x) * 3
			f.Pix[i] = 0.9
			f.Pix[i+1] = 0.9
			f.Pix[i+2] = 0.9
		}
	}
	_ = removeStars(f, 4.0, 10)
	return nil
}

func testRemoveStarsRadius() error {
	const w, h = 100, 100
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.1
	}
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			i := ((50+dy)*w + (50 + dx)) * 3
			f.Pix[i] = 0.9
			f.Pix[i+1] = 0.9
			f.Pix[i+2] = 0.9
		}
	}
	result := removeStars(f, 4.0, 20)
	for _, d := range []int{0, 5, 10, 15} {
		i := (50*w + (50 + d)) * 3
		if result.Pix[i] > 0.3 {
			return fmt.Errorf("star not removed at offset %d: %v", d, result.Pix[i])
		}
	}
	return nil
}

func testRemoveStarsPreservesBackground() error {
	const w, h = 100, 100
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w*h; i++ {
		v := float32(0.1 + 0.05*float64(i%10)/10.0)
		f.Pix[i*3] = v
		f.Pix[i*3+1] = v * 0.9
		f.Pix[i*3+2] = v * 0.8
	}
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			i := ((50+dy)*w + (50 + dx)) * 3
			f.Pix[i] = 0.9
			f.Pix[i+1] = 0.9
			f.Pix[i+2] = 0.9
		}
	}
	result := removeStars(f, 4.0, 15)
	for _, pos := range [][2]int{{5, 5}, {10, 90}, {90, 90}, {80, 20}} {
		i := (pos[0]*w + pos[1]) * 3
		if math.Abs(float64(result.Pix[i]-f.Pix[i])) > 0.01 {
			return fmt.Errorf("background at (%d,%d) changed: %v -> %v",
				pos[0], pos[1], f.Pix[i], result.Pix[i])
		}
	}
	return nil
}

func testDetectStarsSigma() error {
	const w, h = 200, 200
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w*h; i++ {
		noise := float32(0.05 * float64(i%97) / 97.0)
		v := float32(0.1) + noise
		f.Pix[i*3] = v
		f.Pix[i*3+1] = v
		f.Pix[i*3+2] = v
	}
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			i := ((100+dy)*w + (100 + dx)) * 3
			f.Pix[i] = 0.9
			f.Pix[i+1] = 0.9
			f.Pix[i+2] = 0.9
		}
	}
	high := detectStarsSigma(f, 100, 5.0)
	low := detectStarsSigma(f, 100, 3.0)
	if len(low) < len(high) {
		return fmt.Errorf("lower sigma should detect >= stars: %d vs %d", len(low), len(high))
	}
	if len(high) < 1 {
		return fmt.Errorf("5-sigma should detect at least 1 star, got %d", len(high))
	}
	return nil
}

func testPreviewJPGRemoved() error {
	dir, err := os.MkdirTemp("", "skystacker-nopreview-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	const w, h = 32, 32
	for i := 0; i < 2; i++ {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.SetRGBA(x, y, color.RGBA{100, 100, 100, 255})
			}
		}
		path := filepath.Join(dir, fmt.Sprintf("f_%02d.jpg", i))
		fp, err := os.Create(path)
		if err != nil {
			return err
		}
		jpeg.Encode(fp, img, &jpeg.Options{Quality: 95})
		fp.Close()
	}
	if err := runPipeline(context.Background(), runOptions{
		InputDir: dir, Workers: 2, Register: false, StackOnly: true, Scale: 1,
	}); err != nil {
		return err
	}
	previewPath := filepath.Join(dir, "0_skystacker_results", "stacked_preview.jpg")
	if _, err := os.Stat(previewPath); err == nil {
		return fmt.Errorf("stacked_preview.jpg should not exist")
	}
	return nil
}

func testOutputFilesList() error {
	dir, err := os.MkdirTemp("", "skystacker-outfiles-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	const w, h = 32, 32
	for i := 0; i < 2; i++ {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.SetRGBA(x, y, color.RGBA{100, 100, 100, 255})
			}
		}
		path := filepath.Join(dir, fmt.Sprintf("f_%02d.jpg", i))
		fp, err := os.Create(path)
		if err != nil {
			return err
		}
		jpeg.Encode(fp, img, &jpeg.Options{Quality: 95})
		fp.Close()
	}
	if err := runPipeline(context.Background(), runOptions{
		InputDir: dir, Workers: 2, Register: false, StackOnly: true, Scale: 1,
	}); err != nil {
		return err
	}
	outDir := filepath.Join(dir, "0_skystacker_results")
	required := []string{
		"stacked_sum.jpg", "stacked_mean.jpg",
		"stacked_no_correction_mean.jpg",
		"stacked_max.jpg", "stacked_min.jpg",
		"report.txt",
	}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			return fmt.Errorf("missing %s: %w", name, err)
		}
	}
	forbidden := []string{"stacked_preview.jpg", "TIFF"}
	for _, name := range forbidden {
		if _, err := os.Stat(filepath.Join(outDir, name)); err == nil {
			return fmt.Errorf("unexpected %s", name)
		}
	}
	return nil
}

func testResizeFrame() error {
	f := &frame{Width: 8, Height: 8, Pix: make([]float32, 8*8*3)}
	for i := range f.Pix {
		f.Pix[i] = float32(i) / float32(len(f.Pix))
	}
	r := resizeFrame(f, 4, 4)
	if r.Width != 4 || r.Height != 4 {
		return fmt.Errorf("dims %dx%d, want 4x4", r.Width, r.Height)
	}
	if len(r.Pix) != 4*4*3 {
		return fmt.Errorf("len %d, want 48", len(r.Pix))
	}
	return nil
}

func testTiffFlagOnlySum() error {
	dir, err := os.MkdirTemp("", "skystacker-tiff-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	const w, h = 32, 32
	for i := 0; i < 2; i++ {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.SetRGBA(x, y, color.RGBA{100, 100, 100, 255})
			}
		}
		path := filepath.Join(dir, fmt.Sprintf("f_%02d.jpg", i))
		fp, err := os.Create(path)
		if err != nil {
			return err
		}
		jpeg.Encode(fp, img, &jpeg.Options{Quality: 95})
		fp.Close()
	}
	if err := runPipeline(context.Background(), runOptions{
		InputDir: dir, Workers: 2, Register: false, StackOnly: true, Scale: 1,
		CreateTIFF: true,
	}); err != nil {
		return err
	}
	tiffDir := filepath.Join(dir, "0_skystacker_results", "TIFF")
	entries, err := os.ReadDir(tiffDir)
	if err != nil {
		return err
	}
	if len(entries) != 1 {
		return fmt.Errorf("TIFF folder should have 1 file, got %d", len(entries))
	}
	if entries[0].Name() != "stacked_sum.tif" {
		return fmt.Errorf("TIFF contains %q, want stacked_sum.tif", entries[0].Name())
	}
	return nil
}

func testStarStructHasSize() error {
	s := star{X: 1, Y: 2, Flux: 3, Size: 4}
	if s.Size != 4 {
		return fmt.Errorf("Size field not set")
	}
	return nil
}

func testDetectStarsSigmaSinglePixel() error {
	const w, h = 100, 100
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.1
	}
	i := (50*w + 50) * 3
	f.Pix[i], f.Pix[i+1], f.Pix[i+2] = 0.9, 0.9, 0.9
	got := detectStarsSigma(f, 100, 4.0)
	for _, s := range got {
		if math.Abs(s.X-50) < 1 && math.Abs(s.Y-50) < 1 {
			if s.Size < 1 {
				return fmt.Errorf("detected star has Size=0")
			}
			return nil
		}
	}
	return fmt.Errorf("single-pixel star not detected (got %d stars)", len(got))
}

func testRemoveStarsFineStars() error {
	const w, h = 200, 200
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.1
	}
	positions := [][2]int{{20, 20}, {50, 80}, {120, 30}, {180, 190}, {100, 100}, {150, 150}}
	for _, p := range positions {
		i := (p[1]*w + p[0]) * 3
		f.Pix[i], f.Pix[i+1], f.Pix[i+2] = 0.9, 0.9, 0.9
	}
	result := removeStars(f, 4.0, 8)
	for _, p := range positions {
		i := (p[1]*w + p[0]) * 3
		if result.Pix[i] > 0.3 {
			return fmt.Errorf("fine star at %v not removed: %v", p, result.Pix[i])
		}
	}
	return nil
}

func testParseDcrawOutput() error {
	// Realistic outputs from `dcraw -i -v`. Some dcraw builds pad the
	// numbers with spaces inside "%4d x %4d", others don't.
	samples := []struct {
		text string
		w, h int
	}{
		{"Output size: 6000 x 4000\n", 6000, 4000},
		{"Output size:  6000 x  4000\n", 6000, 4000},
		{"Output size: 6024 x 4020\nImage size: 6000 x 4000\n", 6024, 4020},
		{"Image size: 5000 x 3000\n", 5000, 3000},
		{"Full size: 4000 x 2500\n", 4000, 2500},
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`Output size:\s*(\d+)\s*x\s*(\d+)`),
		regexp.MustCompile(`Image size:\s*(\d+)\s*x\s*(\d+)`),
		regexp.MustCompile(`Full size:\s*(\d+)\s*x\s*(\d+)`),
	}
	for i, s := range samples {
		matched := false
		for _, re := range patterns {
			if m := re.FindStringSubmatch(s.text); len(m) == 3 {
				w, _ := strconv.Atoi(m[1])
				h, _ := strconv.Atoi(m[2])
				if w == s.w && h == s.h {
					matched = true
					break
				}
				return fmt.Errorf("sample %d: got %dx%d, want %dx%d", i, w, h, s.w, s.h)
			}
		}
		if !matched {
			return fmt.Errorf("sample %d: no pattern matched %q", i, s.text)
		}
	}
	return nil
}

func testFindCR2Tool() error {
	_ = findCR2Tool()
	return nil
}

func testBlackPointShift() error {
	const w, h = 10, 10
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w*h; i++ {
		v := float32(0.05 + 0.10*float64(i)/float64(w*h))
		f.Pix[i*3] = v
		f.Pix[i*3+1] = v
		f.Pix[i*3+2] = v
	}
	result := blackPointShift(f, 10.0)
	for i := 0; i < w*h/10; i++ {
		if result.Pix[i*3] > 0.01 {
			return fmt.Errorf("darkest pixel %d not zeroed: %v", i, result.Pix[i*3])
		}
	}
	bright := result.Pix[(w*h-1)*3]
	if bright < 0.08 {
		return fmt.Errorf("bright pixel shifted too much: %v", bright)
	}
	return nil
}

func testMTF() error {
	for _, x := range []float64{0.1, 0.25, 0.5, 0.75, 0.9} {
		if got := mtf(0.5, x); math.Abs(got-x) > 1e-9 {
			return fmt.Errorf("MTF(0.5, %v) = %v, want %v", x, got, x)
		}
	}
	for _, x := range []float64{0.05, 0.1, 0.3} {
		for _, target := range []float64{0.15, 0.25, 0.4} {
			m := mtf(target, x)
			back := mtf(m, x)
			if math.Abs(back-target) > 1e-6 {
				return fmt.Errorf("roundtrip MTF(MTF(%v,%v),%v) = %v, want %v", target, x, x, back, target)
			}
		}
	}
	if mtf(0.5, -0.1) != 0 {
		return errors.New("negative input not clamped")
	}
	if mtf(0.5, 1.5) != 1 {
		return errors.New("overflow input not clamped")
	}
	return nil
}

func testAutoStretch() error {
	const w, h = 100, 100
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w*h; i++ {
		v := float32(0.05) + float32(0.001)*float32(i%10)
		f.Pix[i*3] = v
		f.Pix[i*3+1] = v
		f.Pix[i*3+2] = v
	}
	f.Pix[(50*w+50)*3] = 0.9
	f.Pix[(50*w+50)*3+1] = 0.9
	f.Pix[(50*w+50)*3+2] = 0.9

	out := autoStretch(f, -2.8, 0.25, true)
	for i, v := range out.Pix {
		if math.IsNaN(float64(v)) || v < 0 || v > 1.0001 {
			return fmt.Errorf("out-of-range at %d: %v", i, v)
		}
	}
	bgOut := out.Pix[(10*w+10)*3]
	if bgOut < 0.10 || bgOut > 0.40 {
		return fmt.Errorf("background pixel %.3f, want ~0.25", bgOut)
	}
	starOut := out.Pix[(50*w+50)*3]
	if starOut < 0.85 {
		return fmt.Errorf("bright star %.3f, want ~1.0", starOut)
	}
	return nil
}

func testStrictUniformDimensions() error {
	dir, err := os.MkdirTemp("", "skystacker-strict-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	writeJPEG := func(name string, w, h int) error {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.SetRGBA(x, y, color.RGBA{100, 100, 100, 255})
			}
		}
		fp, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		defer fp.Close()
		return jpeg.Encode(fp, img, &jpeg.Options{Quality: 90})
	}
	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		if err := writeJPEG(name, 32, 32); err != nil {
			return err
		}
	}
	if err := writeJPEG("d.jpg", 16, 16); err != nil {
		return err
	}
	err = runPipeline(context.Background(), runOptions{
		InputDir: dir, Workers: 1, Register: false, StackOnly: true, Scale: 1,
	})
	if err == nil {
		return errors.New("expected error for mixed dimensions")
	}
	msg := err.Error()
	if !strings.Contains(msg, "must be exactly the same size") {
		return fmt.Errorf("error message missing required phrase: %v", err)
	}
	if !strings.Contains(msg, "32x32") || !strings.Contains(msg, "16x16") {
		return fmt.Errorf("error message missing size groups: %v", err)
	}
	if !strings.Contains(msg, "(3 file(s))") {
		return fmt.Errorf("error message missing file count for 32x32 group: %v", err)
	}
	if !strings.Contains(msg, "(1 file(s))") {
		return fmt.Errorf("error message missing file count for 16x16 group: %v", err)
	}
	return nil
}

func testSameDimensionsSucceeds() error {
	dir, err := os.MkdirTemp("", "skystacker-same-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	writeJPEG := func(name string, w, h int) error {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				img.SetRGBA(x, y, color.RGBA{100, 100, 100, 255})
			}
		}
		fp, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		defer fp.Close()
		return jpeg.Encode(fp, img, &jpeg.Options{Quality: 90})
	}
	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		if err := writeJPEG(name, 32, 32); err != nil {
			return err
		}
	}
	if err := runPipeline(context.Background(), runOptions{
		InputDir: dir, Workers: 1, Register: false, StackOnly: true, Scale: 1,
	}); err != nil {
		return fmt.Errorf("runPipeline: %w", err)
	}
	// Output folder must be 0_skystacker_results.
	if _, err := os.Stat(filepath.Join(dir, "0_skystacker_results", "report.txt")); err != nil {
		return fmt.Errorf("expected output in 0_skystacker_results: %w", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "skystacker_results")); err == nil {
		return errors.New("old 'skystacker_results' folder should not exist")
	}
	return nil
}

func testHDRBlend() error {
	const w, h = 8, 8
	base := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	detail := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w*h; i++ {
		base.Pix[i*3] = 0.2
		base.Pix[i*3+1] = 0.3
		base.Pix[i*3+2] = 0.4
		detail.Pix[i*3] = 0.1
		detail.Pix[i*3+1] = 0.1
		detail.Pix[i*3+2] = 0.1
	}
	out := blendHDR(base, detail, 1.0)
	for i := 0; i < w*h; i++ {
		if math.Abs(float64(out.Pix[i*3]-0.3)) > 1e-6 {
			return fmt.Errorf("R pixel %d = %v, want 0.3", i, out.Pix[i*3])
		}
		if math.Abs(float64(out.Pix[i*3+1]-0.4)) > 1e-6 {
			return fmt.Errorf("G pixel %d = %v, want 0.4", i, out.Pix[i*3+1])
		}
		if math.Abs(float64(out.Pix[i*3+2]-0.5)) > 1e-6 {
			return fmt.Errorf("B pixel %d = %v, want 0.5", i, out.Pix[i*3+2])
		}
	}
	out = blendHDR(base, detail, 0.5)
	for i := 0; i < w*h; i++ {
		if math.Abs(float64(out.Pix[i*3]-0.25)) > 1e-6 {
			return fmt.Errorf("factor 0.5 R pixel %d = %v, want 0.25", i, out.Pix[i*3])
		}
	}
	out = blendHDR(base, detail, 0)
	for i := 0; i < w*h*3; i++ {
		if out.Pix[i] != base.Pix[i] {
			return fmt.Errorf("factor 0 altered pixel %d", i)
		}
	}
	base.Pix[0], detail.Pix[0] = 0.8, 0.5
	out = blendHDR(base, detail, 1.0)
	if out.Pix[0] != 1.0 {
		return fmt.Errorf("overflow not clamped: %v", out.Pix[0])
	}
	base.Pix[0], detail.Pix[0] = 0.5, -0.3
	out = blendHDR(base, detail, 1.0)
	if math.Abs(float64(out.Pix[0]-0.5)) > 1e-6 {
		return fmt.Errorf("negative detail not clamped: %v", out.Pix[0])
	}
	return nil
}

func testGuidedFilterGray() error {
	const w, h = 16, 16
	uniform := make([]float32, w*h)
	for i := range uniform {
		uniform[i] = 0.5
	}
	out := guidedFilterGray(uniform, w, h, 2, 0.01)
	for i, v := range out {
		if math.Abs(float64(v)-0.5) > 1e-3 {
			return fmt.Errorf("uniform image changed at %d: %v", i, v)
		}
	}
	spotted := make([]float32, w*h)
	for i := range spotted {
		spotted[i] = 0.5
	}
	spotted[(h/2)*w+w/2] = 1.0
	out = guidedFilterGray(spotted, w, h, 2, 0.01)
	center := out[(h/2)*w+w/2]
	if center >= 1.0 {
		return fmt.Errorf("bright spot not smoothed: %v", center)
	}
	if center <= 0.5 {
		return fmt.Errorf("bright spot fully erased: %v", center)
	}
	return nil
}

func testDenoiseFrame() error {
	const w, h = 16, 16
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w*h; i++ {
		v := float32(0.5)
		if i%2 == 0 {
			v = 0.6
		} else {
			v = 0.4
		}
		f.Pix[i*3] = v
		f.Pix[i*3+1] = v
		f.Pix[i*3+2] = v
	}
	out := denoiseFrame(f, 3, 0.01, 1.0)
	beforeDev, afterDev := 0.0, 0.0
	for i := 0; i < w*h; i++ {
		beforeDev += math.Abs(float64(f.Pix[i*3]) - 0.5)
		afterDev += math.Abs(float64(out.Pix[i*3]) - 0.5)
	}
	if afterDev >= beforeDev*0.9 {
		return fmt.Errorf("deviation not reduced: before %.3f after %.3f", beforeDev, afterDev)
	}
	passthru := denoiseFrame(f, 3, 0.01, 0)
	for i := range f.Pix {
		if passthru.Pix[i] != f.Pix[i] {
			return fmt.Errorf("strength 0 altered pixel %d", i)
		}
	}
	return nil
}

func testDenoiseFrameIterations() error {
	const w, h = 32, 32
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	// Checkerboard of +/-0.1 around a 0.5 baseline.
	for i := 0; i < w*h; i++ {
		v := float32(0.5)
		if i%2 == 0 {
			v = 0.6
		} else {
			v = 0.4
		}
		f.Pix[i*3] = v
		f.Pix[i*3+1] = v
		f.Pix[i*3+2] = v
	}
	deviation := func(x *frame) float64 {
		total := 0.0
		for i := 0; i < w*h; i++ {
			total += math.Abs(float64(x.Pix[i*3]) - 0.5)
		}
		return total
	}
	one := denoiseFrame(f, 3, 0.01, 1.0)
	two := denoiseFrame(f, 3, 0.01, 2.0)
	five := denoiseFrame(f, 3, 0.01, 2.5)
	three := denoiseFrame(f, 3, 0.01, 3.0)
	oneDev := deviation(one)
	twoDev := deviation(two)
	fiveDev := deviation(five)
	threeDev := deviation(three)
	if threeDev >= oneDev {
		return fmt.Errorf("strength 3 not smoother than strength 1: %.3f vs %.3f", threeDev, oneDev)
	}
	if !(twoDev >= fiveDev && fiveDev >= threeDev) {
		return fmt.Errorf("monotonicity violated: two=%.3f five=%.3f three=%.3f", twoDev, fiveDev, threeDev)
	}
	pass := denoiseFrame(f, 3, 0.01, 0)
	for i := range f.Pix {
		if pass.Pix[i] != f.Pix[i] {
			return fmt.Errorf("strength 0 altered pixel %d", i)
		}
	}
	return nil
}

func testBlendWeighted() error {
	const w, h = 8, 8
	a := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	b := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range a.Pix {
		a.Pix[i] = 0.2
		b.Pix[i] = 0.8
	}
	out := blendWeighted(a, b, 1.0)
	for i := range out.Pix {
		if math.Abs(float64(out.Pix[i])-0.2) > 1e-6 {
			return fmt.Errorf("wa=1 pixel %d = %v, want 0.2", i, out.Pix[i])
		}
	}
	out = blendWeighted(a, b, 0.0)
	for i := range out.Pix {
		if math.Abs(float64(out.Pix[i])-0.8) > 1e-6 {
			return fmt.Errorf("wa=0 pixel %d = %v, want 0.8", i, out.Pix[i])
		}
	}
	out = blendWeighted(a, b, 0.5)
	for i := range out.Pix {
		if math.Abs(float64(out.Pix[i])-0.5) > 1e-6 {
			return fmt.Errorf("wa=0.5 pixel %d = %v, want 0.5", i, out.Pix[i])
		}
	}
	out = blendWeighted(a, b, 0.25)
	for i := range out.Pix {
		if math.Abs(float64(out.Pix[i])-0.65) > 1e-6 {
			return fmt.Errorf("wa=0.25 pixel %d = %v, want 0.65", i, out.Pix[i])
		}
	}
	a.Pix[0], b.Pix[0] = 0.9, 0.9
	out = blendWeighted(a, b, 0.5)
	if math.Abs(float64(out.Pix[0])-0.9) > 1e-6 {
		return fmt.Errorf("clamping failed: %v", out.Pix[0])
	}
	bad := &frame{Width: w + 1, Height: h, Pix: make([]float32, (w+1)*h*3)}
	out = blendWeighted(a, bad, 0.5)
	if out != a {
		return errors.New("mismatched dims should return first frame")
	}
	return nil
}

func testComputeFrameQuality() error {
	cases := []struct {
		inliers, stars, ref int
		scale, rot          float64
		min, max            float64
	}{
		{100, 100, 100, 1.0, 0.0, 99.9, 100.1},
		{50, 100, 100, 1.0, 0.0, 49.9, 50.1},
		{4, 45, 100, 0.5, 108.0, 0.0, 5.0},
		{0, 0, 0, 1.0, 0.0, 0.0, 0.1},
	}
	for i, c := range cases {
		q := computeFrameQuality(c.inliers, c.stars, c.ref, c.scale, c.rot)
		if q < c.min || q > c.max {
			return fmt.Errorf("case %d: q=%.2f, want [%.2f, %.2f]", i, q, c.min, c.max)
		}
	}
	return nil
}

func testFrameBoxInReference() error {
	minX, minY, maxX, maxY := frameBoxInReference(100, 50, identitySimilarity)
	if minX != 0 || minY != 0 || maxX != 99 || maxY != 49 {
		return fmt.Errorf("identity box = (%d,%d,%d,%d), want (0,0,99,49)", minX, minY, maxX, maxY)
	}
	sim := similarity{SC: 1, SS: 0, Tx: 10, Ty: 20}
	minX, minY, maxX, maxY = frameBoxInReference(100, 50, sim)
	if minX != 10 || minY != 20 || maxX != 109 || maxY != 69 {
		return fmt.Errorf("translated box = (%d,%d,%d,%d), want (10,20,109,69)", minX, minY, maxX, maxY)
	}
	return nil
}

func testMatchStarsRejectsFewInliers() error {
	refStars := make([]star, 9)
	for i := range refStars {
		refStars[i] = star{X: float64(100 + i*50), Y: float64(100 + i*30), Flux: 1000}
	}
	var tgtStars []star
	for i := 0; i < 4; i++ {
		tgtStars = append(tgtStars, star{X: refStars[i].X + 1, Y: refStars[i].Y + 1, Flux: 1000})
	}
	for i := 0; i < 10; i++ {
		tgtStars = append(tgtStars, star{X: float64(2000 + i*100), Y: float64(2000 + i*100), Flux: 100})
	}
	_, inliers, ok := matchStarsToReference(refStars, tgtStars)
	if ok {
		return fmt.Errorf("expected rejection for %d inliers, but got ok=true", inliers)
	}
	return nil
}

func testMedianInPlace() error {
	vals := []float32{3, 1, 4, 1, 5, 9, 2, 6}
	med := medianInPlace(vals)
	// sorted: [1, 1, 2, 3, 4, 5, 6, 9] -> median = (3+4)/2 = 3.5
	if math.Abs(float64(med)-3.5) > 1e-6 {
		return fmt.Errorf("even median = %v, want 3.5", med)
	}
	vals = []float32{5, 1, 3}
	med = medianInPlace(vals)
	if med != 3 {
		return fmt.Errorf("odd median = %v, want 3", med)
	}
	if medianInPlace(nil) != 0 {
		return errors.New("empty median should be 0")
	}
	return nil
}

func testMadOf() error {
	vals := []float32{1, 2, 3, 4, 5}
	mad := madOf(vals, 3)
	if math.Abs(float64(mad)-1.0) > 1e-6 {
		return fmt.Errorf("mad = %v, want 1", mad)
	}
	return nil
}

func testSigmaClipReduce() error {
	vals := []float32{5, 5.1, 4.9, 5.05, 4.95, 5.02, 4.98, 5.01, 4.99, 5.03, 100, -50}
	mean, _, acc, rej := sigmaClipReduce(vals, 3.0, 2)
	if math.Abs(float64(mean)-5.0) > 0.5 {
		return fmt.Errorf("clipped mean = %v, want ~5", mean)
	}
	if rej < 2 {
		return fmt.Errorf("rejected = %d, want >= 2", rej)
	}
	if acc+rej != len(vals) {
		return fmt.Errorf("acc+rej = %d, want %d", acc+rej, len(vals))
	}
	return nil
}

func testSigmaClipAllSame() error {
	vals := []float32{0.5, 0.5, 0.5, 0.5}
	mean, _, acc, rej := sigmaClipReduce(vals, 3.0, 2)
	if math.Abs(float64(mean)-0.5) > 1e-6 {
		return fmt.Errorf("mean = %v, want 0.5", mean)
	}
	if acc != 4 || rej != 0 {
		return fmt.Errorf("acc=%d rej=%d, want 4, 0", acc, rej)
	}
	return nil
}

func testSigmaClipReduceWeighted() error {
	// Basic: asymmetric weights should pull the mean toward the
	// heavily-weighted values. vals={0.4, 0.6, 0.5} with weights
	// {0.9, 0.1, 0.9}: weighted mean is (0.36 + 0.06 + 0.45)/1.9
	// = 0.87/1.9 ~= 0.4579, while unweighted is 0.5.
	vals := []float32{0.4, 0.6, 0.5}
	weights := []float32{0.9, 0.1, 0.9}
	mean, _, acc, rej := sigmaClipReduceWeighted(vals, weights, 3.0, 1)
	want := (0.4*0.9 + 0.6*0.1 + 0.5*0.9) / (0.9 + 0.1 + 0.9)
	if math.Abs(float64(mean)-want) > 1e-3 {
		return fmt.Errorf("weighted mean = %v, want %v", mean, want)
	}
	if acc+rej != len(vals) {
		return fmt.Errorf("acc+rej = %d, want %d", acc+rej, len(vals))
	}

	// Uniform weights must give the same result as unweighted.
	vals2 := []float32{5, 5.1, 4.9, 5.05, 4.95, 5.02, 4.98, 5.01, 4.99, 5.03, 100, -50}
	w2 := make([]float32, len(vals2))
	for i := range w2 {
		w2[i] = 1.0
	}
	mUnw, _, accUnw, rejUnw := sigmaClipReduce(vals2, 3.0, 2)
	mW, _, accW, rejW := sigmaClipReduceWeighted(vals2, w2, 3.0, 2)
	if math.Abs(float64(mUnw-mW)) > 1e-4 {
		return fmt.Errorf("uniform-weighted mean %v != unweighted %v", mW, mUnw)
	}
	if accUnw != accW || rejUnw != rejW {
		return fmt.Errorf("uniform-weighted acc/rej = %d/%d, want %d/%d",
			accW, rejW, accUnw, rejUnw)
	}

	// All-zero weights must fall back to the unweighted mean of
	// the survivors, not divide by zero and return 0.
	zW := make([]float32, len(vals2))
	mZero, _, _, _ := sigmaClipReduceWeighted(vals2, zW, 3.0, 2)
	if math.Abs(float64(mZero)-float64(mUnw)) > 1e-4 {
		return fmt.Errorf("zero-weighted mean %v != unweighted %v", mZero, mUnw)
	}
	return nil
}

func testReadTileFromRaw() error {
	dir, err := os.MkdirTemp("", "skystacker-tile-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	const w, h = 16, 16
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			f.Pix[i] = float32(y*w+x) / float32(w*h)
			f.Pix[i+1] = f.Pix[i] * 2
			f.Pix[i+2] = f.Pix[i] * 3
		}
	}
	path := filepath.Join(dir, "t.f32raw")
	if err := writeSpillFrame(path, f); err != nil {
		return err
	}
	tileW, tileH := 4, 4
	dst := make([]float32, tileW*tileH*3)
	if err := readTileFromRaw(path, w, h, 4, 4, 8, 8, dst); err != nil {
		return err
	}
	for py := 0; py < tileH; py++ {
		for px := 0; px < tileW; px++ {
			origI := ((4+py)*w + (4 + px)) * 3
			dstI := (py*tileW + px) * 3
			if math.Abs(float64(dst[dstI]-f.Pix[origI])) > 1e-9 {
				return fmt.Errorf("tile mismatch at (%d,%d)", px, py)
			}
		}
	}
	return nil
}

func testReduceTilesStack() error {
	dir, err := os.MkdirTemp("", "skystacker-reduce-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	const w, h = 32, 32
	var spillPaths []string
	for i := 0; i < 5; i++ {
		f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
		for j := range f.Pix {
			f.Pix[j] = 0.5
		}
		if i == 3 {
			for j := range f.Pix {
				f.Pix[j] = 0.55
			}
		}
		if i == 4 {
			f.Pix[(5*w+5)*3] = 1.0
			f.Pix[(10*w+10)*3] = 0.0
		}
		path := filepath.Join(dir, fmt.Sprintf("frame_%d.f32raw", i))
		if err := writeSpillFrame(path, f); err != nil {
			return err
		}
		spillPaths = append(spillPaths, path)
	}
	sumPix, meanPix, rejPix, wgtPix, err := reduceTilesStack(
		spillPaths, w, h, image.Rect(0, 0, w, h),
		"sigma", 3.0, 2, 16, 1, nil)
	if err != nil {
		return err
	}
	if math.Abs(float64(meanPix[0])-0.51) > 0.02 {
		return fmt.Errorf("mean at (0,0) = %v, want ~0.51", meanPix[0])
	}
	if math.Abs(float64(meanPix[(5*w+5)*3])-0.51) > 0.02 {
		return fmt.Errorf("mean at (5,5) = %v, want ~0.51 (outlier rejected)", meanPix[(5*w+5)*3])
	}
	if rejPix[5*w+5] < 1 {
		return fmt.Errorf("rejection at (5,5) = %v, want >= 1", rejPix[5*w+5])
	}
	// Frame 3 is uniformly 0.55 in this fixture, so at (0,0)
	// it is correctly rejected by sigma clipping: MAD=0 (four of
	// five values agree exactly), sigma is floored to 1e-9, the
	// threshold is 3e-9, and 0.05 >> 3e-9. That leaves 4 accepted
	// frames, so weight = 4, not 5.
	if wgtPix[0] < 3.5 {
		return fmt.Errorf("weight at (0,0) = %v, want >= 4", wgtPix[0])
	}
	if sumPix == nil || meanPix == nil || rejPix == nil || wgtPix == nil {
		return errors.New("nil output slice")
	}
	return nil
}

func testReduceTilesStackWeighted() error {
	dir, err := os.MkdirTemp("", "skystacker-reduce-w-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	const w, h = 16, 16
	// Five frames spanning 0.3..0.7 with the far-right frame (0.7)
	// down-weighted to 0.1. Unweighted mean would be 0.5; weighted
	// mean is ~0.4568. Sigma clipping with threshold=3 must NOT reject
	// anything (all deviations are within 3*MAD).
	vals := []float32{0.3, 0.4, 0.5, 0.6, 0.7}
	weights := []float32{0.9, 0.9, 0.9, 0.9, 0.1}
	var spillPaths []string
	for i, v := range vals {
		f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
		for j := range f.Pix {
			f.Pix[j] = v
		}
		path := filepath.Join(dir, fmt.Sprintf("frame_%d.f32raw", i))
		if err := writeSpillFrame(path, f); err != nil {
			return err
		}
		spillPaths = append(spillPaths, path)
	}
	_, meanW, _, _, err := reduceTilesStack(
		spillPaths, w, h, image.Rect(0, 0, w, h),
		"sigma", 3.0, 2, 16, 1, weights)
	if err != nil {
		return err
	}
	wantW := (0.3*0.9 + 0.4*0.9 + 0.5*0.9 + 0.6*0.9 + 0.7*0.1) / (0.9*4 + 0.1)
	if math.Abs(float64(meanW[0])-wantW) > 0.01 {
		return fmt.Errorf("weighted mean = %v, want ~%v", meanW[0], wantW)
	}
	_, meanUnw, _, _, err := reduceTilesStack(
		spillPaths, w, h, image.Rect(0, 0, w, h),
		"sigma", 3.0, 2, 16, 1, nil)
	if err != nil {
		return err
	}
	if math.Abs(float64(meanUnw[0])-0.5) > 0.02 {
		return fmt.Errorf("unweighted mean = %v, want ~0.5 (fixture must not clip)", meanUnw[0])
	}
	if math.Abs(float64(meanW[0]-meanUnw[0])) < 0.02 {
		return fmt.Errorf("weighted %v and unweighted %v differ by <0.02; test is not sensitive",
			meanW[0], meanUnw[0])
	}
	return nil
}

func testChromaDenoiseGB() error {
	const w, h = 64, 64
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	// Build an image with uniform luminance Y=0.5 but chroma that
	// follows a sinusoidal spatial pattern at ~20 px scale. This is
	// the synthetic analogue of "color blotch" noise in the sky
	// background of a stretched astro frame.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			cb := 0.10 * math.Sin(float64(x+y)/20*2*math.Pi)
			cr := 0.10 * math.Sin(float64(x-y)/20*2*math.Pi)
			y0 := 0.5
			r := y0 + cr
			b := y0 + cb
			g := y0 - 0.5094*cr - 0.1942*cb
			f.Pix[i] = float32(clampFloat(r, 0, 1))
			f.Pix[i+1] = float32(clampFloat(g, 0, 1))
			f.Pix[i+2] = float32(clampFloat(b, 0, 1))
		}
	}
	out := chromaDenoiseGB(f, 24, 0.02, 0.7, 0.25)
	stddev := func(v []float64) float64 {
		var sum, sq float64
		for _, x := range v {
			sum += x
			sq += x * x
		}
		n := float64(len(v))
		mean := sum / n
		return math.Sqrt(sq/n - mean*mean)
	}
	var origCb, afterCb, origY, afterY []float64
	for i := 0; i < w*h; i++ {
		r0 := float64(f.Pix[i*3])
		g0 := float64(f.Pix[i*3+1])
		b0 := float64(f.Pix[i*3+2])
		yI0 := 0.299*r0 + 0.587*g0 + 0.114*b0
		origY = append(origY, yI0)
		origCb = append(origCb, b0-yI0)
		r1 := float64(out.Pix[i*3])
		g1 := float64(out.Pix[i*3+1])
		b1 := float64(out.Pix[i*3+2])
		yI1 := 0.299*r1 + 0.587*g1 + 0.114*b1
		afterY = append(afterY, yI1)
		afterCb = append(afterCb, b1-yI1)
	}
	origCbSD := stddev(origCb)
	afterCbSD := stddev(afterCb)
	origYSD := stddev(origY)
	afterYSD := stddev(afterY)
	if origCbSD < 0.03 {
		return fmt.Errorf("test fixture: original chroma stddev %.4f too small", origCbSD)
	}
	// Chroma noise must drop by at least 40%.
	if afterCbSD >= 0.6*origCbSD {
		return fmt.Errorf("chroma stddev reduced only from %.4f to %.4f", origCbSD, afterCbSD)
	}
	// Luminance should stay essentially flat (original is already flat).
	if origYSD > 0.02 {
		return fmt.Errorf("test fixture: original luminance stddev %.4f too large", origYSD)
	}
	if afterYSD > 0.03 {
		return fmt.Errorf("luminance stddev grew to %.4f (should stay near 0)", afterYSD)
	}
	return nil
}

func testChromaDenoiseMultiScale() error {
	const w, h = 128, 128
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	// Uniform luminance Y=0.5, with sinusoidal chroma at 20 px and
	// 60 px scales. This is the synthetic analogue of the low-freq
	// red/green blotchiness that survives the small-radius passes.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			cb := 0.06*math.Sin(float64(x+y)/20*2*math.Pi) + 0.06*math.Sin(float64(x-y)/60*2*math.Pi)
			cr := 0.06*math.Sin(float64(x-y)/20*2*math.Pi) + 0.06*math.Sin(float64(x+y)/60*2*math.Pi)
			yb := 0.5
			r := yb + cr
			b := yb + cb
			g := yb - 0.5094*cr - 0.1942*cb
			f.Pix[i] = float32(clampFloat(r, 0, 1))
			f.Pix[i+1] = float32(clampFloat(g, 0, 1))
			f.Pix[i+2] = float32(clampFloat(b, 0, 1))
		}
	}
	stddev := func(v []float64) float64 {
		var sum, sq float64
		for _, x := range v {
			sum += x
			sq += x * x
		}
		n := float64(len(v))
		m := sum / n
		return math.Sqrt(sq/n - m*m)
	}
	beforeCb := make([]float64, 0, w*h)
	beforeY := make([]float64, 0, w*h)
	for i := 0; i < w*h; i++ {
		r := float64(f.Pix[i*3])
		g := float64(f.Pix[i*3+1])
		b := float64(f.Pix[i*3+2])
		yv := 0.299*r + 0.587*g + 0.114*b
		beforeY = append(beforeY, yv)
		beforeCb = append(beforeCb, b-yv)
	}
	out := chromaDenoiseMultiScale(f, 1.0)
	afterCb := make([]float64, 0, w*h)
	afterY := make([]float64, 0, w*h)
	for i := 0; i < w*h; i++ {
		r := float64(out.Pix[i*3])
		g := float64(out.Pix[i*3+1])
		b := float64(out.Pix[i*3+2])
		yv := 0.299*r + 0.587*g + 0.114*b
		afterY = append(afterY, yv)
		afterCb = append(afterCb, b-yv)
	}
	beforeCbSD := stddev(beforeCb)
	afterCbSD := stddev(afterCb)
	beforeYSD := stddev(beforeY)
	afterYSD := stddev(afterY)
	if beforeCbSD < 0.03 {
		return fmt.Errorf("fixture: chroma stddev %.4f too small", beforeCbSD)
	}
	if afterCbSD >= 0.6*beforeCbSD {
		return fmt.Errorf("chroma noise reduced only from %.4f to %.4f", beforeCbSD, afterCbSD)
	}
	if beforeYSD > 0.02 {
		return fmt.Errorf("fixture: luminance stddev %.4f too large", beforeYSD)
	}
	if afterYSD > 0.03 {
		return fmt.Errorf("luminance stddev grew to %.4f", afterYSD)
	}
	pass := chromaDenoiseMultiScale(f, 0)
	if pass != f {
		return fmt.Errorf("strength 0 should return f unchanged")
	}
	// Iteration must be strictly more aggressive: strength=2 should leave
	// less chroma residual than strength=1.
	out2 := chromaDenoiseMultiScale(f, 2.0)
	afterCb2 := make([]float64, 0, w*h)
	for i := 0; i < w*h; i++ {
		r := float64(out2.Pix[i*3])
		g := float64(out2.Pix[i*3+1])
		b := float64(out2.Pix[i*3+2])
		yv := 0.299*r + 0.587*g + 0.114*b
		afterCb2 = append(afterCb2, b-yv)
	}
	afterCb2SD := stddev(afterCb2)
	if afterCb2SD >= afterCbSD {
		return fmt.Errorf("strength=2 chroma stddev %.4f not below strength=1 %.4f",
			afterCb2SD, afterCbSD)
	}
	return nil
}

func testApplyCurves() error {
	// w must be odd so the midpoint pixel lands exactly at 0.5: mid =
	// (w-1)/2, and mid/(w-1) == 0.5 iff w-1 is even, i.e. w is odd.
	// Earlier versions used w=32 and asserted ((0.5-0.03)/0.97)^1.2,
	// but the mid pixel was at 15/31 ~= 0.48387, so the got/want values
	// differed by ~0.017 and the test failed once it was actually run.
	const w, h = 33, 1
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w; i++ {
		v := float32(i) / float32(w-1)
		f.Pix[i*3] = v
		f.Pix[i*3+1] = v
		f.Pix[i*3+2] = v
	}
	out := applyCurves(f, 0.03, 1.20)
	// 0.0 -> 0 (crushed to black by `black=0.03`)
	if out.Pix[0] != 0 {
		return fmt.Errorf("black=0 input should map to 0, got %v", out.Pix[0])
	}
	// 1.0 -> 1 (highlight preserved)
	last := out.Pix[(w-1)*3]
	if math.Abs(float64(last)-1.0) > 1e-6 {
		return fmt.Errorf("white should stay 1, got %v", last)
	}
	// 0.5 -> ((0.5-0.03)/0.97)^1.20 ~= 0.419
	mid := (w - 1) / 2
	gotMid := out.Pix[mid*3]
	wantMid := float32(math.Pow((0.5-0.03)/0.97, 1.20))
	if math.Abs(float64(gotMid-wantMid)) > 1e-4 {
		return fmt.Errorf("midtone = %v, want %v", gotMid, wantMid)
	}
	if gotMid >= 0.5 {
		return fmt.Errorf("midtone %v not darkened below 0.5", gotMid)
	}
	// Neutral params return f unchanged
	same := applyCurves(f, 0, 1.0)
	for i := range f.Pix {
		if same.Pix[i] != f.Pix[i] {
			return fmt.Errorf("neutral params altered pixel %d", i)
		}
	}
	// Out-of-range values are clamped
	f.Pix[0] = -0.5
	f.Pix[1] = -0.5
	f.Pix[2] = -0.5
	f.Pix[3] = 1.5
	f.Pix[4] = 1.5
	f.Pix[5] = 1.5
	out = applyCurves(f, 0.03, 1.20)
	for i := 0; i < 6; i++ {
		if out.Pix[i] < 0 || out.Pix[i] > 1 {
			return fmt.Errorf("output %d out of range: %v", i, out.Pix[i])
		}
	}
	return nil
}

func testSpikesMinCount() error {
	const w, h = 200, 200
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.05
	}
	// Three bright stars — below the minCount floor.
	for k := 0; k < 3; k++ {
		cx := 30 + k*60
		cy := 100
		for c := 0; c < 3; c++ {
			f.Pix[(cy*w+cx)*3+c] = 0.9
		}
	}
	noFloor := selectSpikeStars(f, 0.2, 0)
	if len(noFloor) != 1 {
		return fmt.Errorf("no floor: got %d, want 1 (percent floor)", len(noFloor))
	}
	withFloor5 := selectSpikeStars(f, 0.2, 5)
	if len(withFloor5) != 3 {
		return fmt.Errorf("floor=5, 3 stars: got %d, want 3 (capped to available)", len(withFloor5))
	}
	withFloor2 := selectSpikeStars(f, 0.2, 2)
	if len(withFloor2) != 2 {
		return fmt.Errorf("floor=2: got %d, want 2", len(withFloor2))
	}
	// Dense field: 100 stars * 5% = 5, floor=5 -> still 5.
	dense := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range dense.Pix {
		dense.Pix[i] = 0.05
	}
	for k := 0; k < 100; k++ {
		cx := 20 + (k%10)*18
		cy := 20 + (k/10)*18
		for c := 0; c < 3; c++ {
			dense.Pix[(cy*w+cx)*3+c] = 0.9
		}
	}
	denseSame := selectSpikeStars(dense, 5.0, 5)
	if len(denseSame) != 5 {
		return fmt.Errorf("dense floor=5: got %d, want 5", len(denseSame))
	}
	denseFloored := selectSpikeStars(dense, 5.0, 10)
	if len(denseFloored) != 10 {
		return fmt.Errorf("dense floor=10: got %d, want 10", len(denseFloored))
	}
	return nil
}

func testAddDiffractionSpikes() error {
	// --- Part 1: selectSpikeStars percent-based selection ---
	const w, h = 200, 200
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.05
	}
	// 20 stars in a grid (2 rows x 10 cols), each a single bright pixel.
	// Slight brightness variation keeps the flux-sort well-defined.
	for k := 0; k < 20; k++ {
		cx := 20 + (k%10)*16
		cy := 20 + (k/10)*100
		v := float32(0.9) - float32(k)*0.03
		for c := 0; c < 3; c++ {
			f.Pix[(cy*w+cx)*3+c] = v
		}
	}

	all := selectSpikeStars(f, 100.0, 0)
	if len(all) < 15 {
		return fmt.Errorf("percent=100: expected ~20, got %d", len(all))
	}
	if n := selectSpikeStars(f, 0, 0); n != nil {
		return fmt.Errorf("percent=0: expected nil, got %d", len(n))
	}
	one := selectSpikeStars(f, 1.0, 0)
	if len(one) != 1 {
		return fmt.Errorf("percent=1: expected 1 (floored), got %d", len(one))
	}
	if all[0].Flux < all[len(all)-1].Flux {
		return fmt.Errorf("stars not sorted by decreasing flux")
	}

	// --- Part 2: overlaySpikes at baseAngle=0 (arms at 0/90/180/270 deg) ---
	out := overlaySpikes(f, all[:1], 20, 0.7, 4, 0, 1.0, 0.0, 1.0, 0.0)
	bx := int(all[0].X + 0.5)
	by := int(all[0].Y + 0.5)
	right := out.Pix[(by*w+bx+5)*3]
	if right <= 0.05 {
		return fmt.Errorf("horizontal spike not visible at +5px: %v", right)
	}
	up := out.Pix[((by-5)*w+bx)*3]
	if up <= 0.05 {
		return fmt.Errorf("vertical spike not visible at -5px: %v", up)
	}
	diag := out.Pix[((by+5)*w+bx+5)*3]
	if diag > 0.10 {
		return fmt.Errorf("diagonal should not be spiked with 4 arms at baseAngle=0: %v", diag)
	}

	// --- Part 3: rotate baseAngle to pi/4, diagonal now spiked ---
	out45 := overlaySpikes(f, all[:1], 20, 0.7, 4, math.Pi/4, 1.0, 0.0, 1.0, 0.0)
	diag45 := out45.Pix[((by+5)*w+bx+5)*3]
	if diag45 <= 0.05 {
		return fmt.Errorf("diagonal should be spiked at baseAngle=pi/4: %v", diag45)
	}

	// --- Part 4: proportional intensity ---
	f2 := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := range f2.Pix {
		f2.Pix[i] = 0.05
	}
	// Bright star at (50, 50), dim star at (150, 50). Both single pixels.
	for c := 0; c < 3; c++ {
		f2.Pix[(50*w+50)*3+c] = 0.9
		f2.Pix[(50*w+150)*3+c] = 0.3
	}
	stars2 := selectSpikeStars(f2, 100.0, 0)
	if len(stars2) != 2 {
		return fmt.Errorf("expected 2 stars, got %d", len(stars2))
	}
	out2 := overlaySpikes(f2, stars2, 20, 0.7, 4, 0, 1.0, 0.0, 1.0, 0.0)
	// stars2[0] is the brighter star at (50, 50); spike at (55, 50).
	// stars2[1] is the dimmer star at (150, 50); spike at (155, 50).
	bSpike := out2.Pix[(50*w+55)*3]
	dSpike := out2.Pix[(50*w+155)*3]
	if bSpike <= dSpike {
		return fmt.Errorf("brighter star spike (%.3f) should exceed dimmer (%.3f)",
			bSpike, dSpike)
	}
	if bSpike < 0.4 {
		return fmt.Errorf("bright star spike too dim: %.3f", bSpike)
	}
	if dSpike > 0.3 {
		return fmt.Errorf("dim star spike too bright: %.3f", dSpike)
	}

	// --- Part 5: softness compresses the dim/bright ratio ---
	// With softness=0.5, the dimmer star's spike should be a larger
	// fraction of the brighter star's than with linear falloff.
	outSoft := overlaySpikes(f2, stars2, 20, 0.7, 4, 0, 0.5, 0.0, 1.0, 0.0)
	bSpikeSoft := outSoft.Pix[(50*w+55)*3]
	dSpikeSoft := outSoft.Pix[(50*w+155)*3]
	if bSpikeSoft <= 0 {
		return fmt.Errorf("bright star spike missing in softness test")
	}
	ratioLinear := dSpike / bSpike
	ratioSoft := dSpikeSoft / bSpikeSoft
	if ratioSoft <= ratioLinear {
		return fmt.Errorf("softness=0.5 should increase dim/bright ratio: linear=%.3f soft=%.3f",
			ratioLinear, ratioSoft)
	}

	// --- Part 6: minRatio floor lifts the faintest star ---
	// Force a high floor so the dim star gets lifted to it.
	outFloor := overlaySpikes(f2, stars2, 20, 0.7, 4, 0, 1.0, 0.8, 1.0, 0.0)
	dSpikeFloor := outFloor.Pix[(50*w+155)*3]
	if dSpikeFloor <= dSpike {
		return fmt.Errorf("minRatio=0.8 should lift dim spike: floor=%.3f linear=%.3f",
			dSpikeFloor, dSpike)
	}

	// --- Part 7: lengthSoftness controls arm length differentiation ---
	// Fixture f2: bright star at (50,50) with flux ~0.85, dim star at
	// (150,50) with flux ~0.25, so relFlux ~ 0.29. baseLength = 50.
	//
	//   lengthSoftness = 1.0 (proportional): dim arm reaches 50*0.29 = 14.5 px
	//   lengthSoftness = 2.0 (strongly super-linear): dim arm reaches 50*0.29^2 = 4.2 px
	//
	// So a pixel at (160, 50) (10 px right of the dim star) should be
	// spiked in the linear case but NOT spiked in the super-linear case.
	outLenLinear := overlaySpikes(f2, stars2, 50, 0.7, 4, 0, 1.0, 0.0, 1.0, 0.0)
	outLenSuper := overlaySpikes(f2, stars2, 50, 0.7, 4, 0, 1.0, 0.0, 2.0, 0.0)
	linearAt10 := outLenLinear.Pix[(50*w+160)*3]
	superAt10 := outLenSuper.Pix[(50*w+160)*3]
	if linearAt10 <= 0.10 {
		return fmt.Errorf("linear length: dim spike should reach 10px from dim star, got %.3f",
			linearAt10)
	}
	if superAt10 >= 0.10 {
		return fmt.Errorf("super-linear length: dim spike should NOT reach 10px from dim star, got %.3f",
			superAt10)
	}
	// And at 5px the dim spike should be spiked in BOTH cases
	// (dim arm reaches 14.5 with linear, 4.2 with super-linear — wait, 5 > 4.2!
	// Let me use distance 2 for the "both spiked" check to be safe.
	linearAt2 := outLenLinear.Pix[(50*w+152)*3]
	superAt2 := outLenSuper.Pix[(50*w+152)*3]
	if linearAt2 <= 0.10 {
		return fmt.Errorf("linear length: dim spike should reach 2px from dim star, got %.3f",
			linearAt2)
	}
	if superAt2 <= 0.10 {
		return fmt.Errorf("super-linear length: dim spike should still reach 2px from dim star, got %.3f",
			superAt2)
	}

	return nil
}

func testEstimateAffine() error {
	src := []point2D{
		{0, 0}, {10, 0}, {0, 10}, {10, 10}, {5, 5}, {3, 7}, {8, 2},
	}
	theta := 5.0 * math.Pi / 180
	c, s := math.Cos(theta), math.Sin(theta)
	a := affine{
		A: 1.02 * c,
		B: -1.02*s + 0.02,
		C: 2,
		D: 1.02 * s,
		E: 1.02 * c,
		F: 3,
	}
	dst := make([]point2D, len(src))
	for i, p := range src {
		dst[i] = a.apply(p)
	}
	got, ok := estimateAffine(src, dst)
	if !ok {
		return errors.New("estimateAffine returned ok=false")
	}
	for i := range src {
		want := a.apply(src[i])
		g := got.apply(src[i])
		if math.Hypot(g.X-want.X, g.Y-want.Y) > 1e-6 {
			return fmt.Errorf("point %d: got %v, want %v", i, g, want)
		}
	}
	if _, ok := estimateAffine(src[:2], dst[:2]); ok {
		return errors.New("estimateAffine accepted <3 points")
	}
	line := []point2D{{0, 0}, {1, 1}, {2, 2}, {3, 3}}
	if _, ok := estimateAffine(line, line); ok {
		return errors.New("estimateAffine accepted collinear points")
	}
	return nil
}

func testAffineInverseRoundTrip() error {
	a := affine{A: 1.1, B: -0.2, C: 5, D: 0.3, E: 0.9, F: -3}
	inv := a.inverse()
	for _, p := range []point2D{{0, 0}, {10, 5}, {-5, 20}} {
		q := a.apply(p)
		back := inv.apply(q)
		if math.Hypot(back.X-p.X, back.Y-p.Y) > 1e-9 {
			return fmt.Errorf("roundtrip %v -> %v -> %v", p, q, back)
		}
	}
	rot := affine{
		A: math.Cos(0.3), B: -math.Sin(0.3), C: 0,
		D: math.Sin(0.3), E: math.Cos(0.3), F: 0,
	}
	if math.Abs(rot.scale()-1.0) > 1e-9 {
		return fmt.Errorf("unit rotation scale = %v, want 1", rot.scale())
	}
	if math.Abs(rot.rotation()-0.3) > 1e-9 {
		return fmt.Errorf("rotation = %v, want 0.3", rot.rotation())
	}
	return nil
}

func testHighPassRadiusAdaptive() error {
	cases := []struct{ w, h, want int }{
		{500, 500, 8},
		{1000, 1000, 8},
		{2000, 2000, 10},
		{4000, 3000, 15},
		{8000, 6000, 24},
		{100, 100, 8},
	}
	for i, c := range cases {
		got := estimateHighPassRadius(c.w, c.h)
		if got != c.want {
			return fmt.Errorf("case %d (%dx%d): got %d, want %d", i, c.w, c.h, got, c.want)
		}
	}
	return nil
}

func testDiskSpaceGuard() error {
	// mean: no disk demand
	if got := estimateDiskDemand("mean", 100, 100*100*3*4); got != 0 {
		return fmt.Errorf("mean demand = %d, want 0", got)
	}
	// median/sigma: 2× frameCount×frameBytes + 500 MB headroom
	for _, m := range []string{"median", "sigma"} {
		const w, h, n = 100, 100, 10
		fb := int64(w) * int64(h) * 3 * 4
		want := int64(n)*fb*2 + 500*1024*1024
		if got := estimateDiskDemand(m, n, fb); got != want {
			return fmt.Errorf("%s demand = %d, want %d", m, got, want)
		}
	}
	// Demand must be non-negative for all methods.
	for _, m := range []string{"mean", "median", "sigma"} {
		if got := estimateDiskDemand(m, 0, 0); got < 0 {
			return fmt.Errorf("%s demand = %d, want >= 0", m, got)
		}
	}
	// getAvailableDiskBytes on a real path should return > 0
	dir, err := os.MkdirTemp("", "skystacker-disk-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	free, err := getAvailableDiskBytes(dir)
	if err != nil {
		return fmt.Errorf("statfs on %s: %w", dir, err)
	}
	if free <= 0 {
		return fmt.Errorf("free bytes = %d, want > 0", free)
	}
	return nil
}

func testComputeNormStats() error {
	f := &frame{Width: 64, Height: 64, Pix: make([]float32, 64*64*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.3
	}
	s := computeNormStats(f)
	if math.Abs(s.Median-0.3) > 0.01 {
		return fmt.Errorf("uniform median = %v, want ~0.3", s.Median)
	}
	if s.MAD > 0.01 {
		return fmt.Errorf("uniform MAD = %v, want ~0", s.MAD)
	}
	return nil
}

func testGlobalNormalizeOffset() error {
	const w, h = 64, 64
	ref := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	target := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			base := float32(0.4 + 0.15*float64((x*7+y*13)%17)/17.0)
			i := (y*w + x) * 3
			ref.Pix[i] = base
			ref.Pix[i+1] = base
			ref.Pix[i+2] = base
			target.Pix[i] = base + 0.1
			target.Pix[i+1] = base + 0.1
			target.Pix[i+2] = base + 0.1
		}
	}
	refStats := computeNormStats(ref)
	targetN := globalNormalizeWithRef(target, refStats)
	for _, p := range [][2]int{{10, 10}, {32, 32}, {50, 20}} {
		x, y := p[0], p[1]
		i := (y*w + x) * 3
		want := ref.Pix[i]
		got := targetN.Pix[i]
		if math.Abs(float64(got-want)) > 0.01 {
			return fmt.Errorf("global normalize at (%d,%d): got %.4f, want ~%.4f", x, y, got, want)
		}
	}
	return nil
}

func testGlobalNormalizeIdentity() error {
	const w, h = 32, 32
	ref := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			base := float32(0.3 + 0.2*float64((x*3+y*7)%13)/13.0)
			i := (y*w + x) * 3
			ref.Pix[i] = base
			ref.Pix[i+1] = base
			ref.Pix[i+2] = base
		}
	}
	refStats := computeNormStats(ref)
	normalized := globalNormalizeWithRef(ref, refStats)
	for _, p := range [][2]int{{5, 5}, {20, 20}, {15, 25}} {
		x, y := p[0], p[1]
		i := (y*w + x) * 3
		if math.Abs(float64(normalized.Pix[i]-ref.Pix[i])) > 0.01 {
			return fmt.Errorf("identity normalize at (%d,%d): got %.4f, want %.4f", x, y, normalized.Pix[i], ref.Pix[i])
		}
	}
	return nil
}

func testLocalNormalizeGradient() error {
	const w, h = 128, 128
	ref := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	target := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			noise := float32(0.01 * float64((x*7+y*13)%17) / 17.0)
			base := float32(0.4) + noise
			i := (y*w + x) * 3
			ref.Pix[i] = base
			ref.Pix[i+1] = base
			ref.Pix[i+2] = base
			offset := float32(0.2 * float64(x) / float64(w))
			tv := base + offset
			target.Pix[i] = tv
			target.Pix[i+1] = tv
			target.Pix[i+2] = tv
		}
	}
	ts := 64
	ntx := (w + ts - 1) / ts
	nty := (h + ts - 1) / ts
	refTiles := make([]normStats, ntx*nty)
	for ty := 0; ty < nty; ty++ {
		for tx := 0; tx < ntx; tx++ {
			x0, y0 := tx*ts, ty*ts
			x1, y1 := x0+ts, y0+ts
			if x1 > w {
				x1 = w
			}
			if y1 > h {
				y1 = h
			}
			refTiles[ty*ntx+tx] = computeRegionStats(ref, x0, y0, x1, y1)
		}
	}
	refGlobal := computeNormStats(ref)
	targetN := localNormalizeWithRefTiles(target, refTiles, refGlobal, ntx, nty, ts)
	for _, p := range [][2]int{{32, 32}, {96, 32}, {32, 96}, {96, 96}} {
		x, y := p[0], p[1]
		i := (y*w + x) * 3
		want := ref.Pix[i]
		got := targetN.Pix[i]
		if math.Abs(float64(got-want)) > 0.02 {
			return fmt.Errorf("local normalize at (%d,%d): got %.4f, want ~%.4f", x, y, got, want)
		}
	}
	return nil
}

func testLocalNormalizeMaskedPixels() error {
	const w, h = 32, 32
	ref := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	target := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			noise := float32(0.02 * float64((x*7+y*13)%11) / 11.0)
			v := float32(0.5) + noise
			i := (y*w + x) * 3
			ref.Pix[i] = v
			ref.Pix[i+1] = v
			ref.Pix[i+2] = v
			tv := v + 0.1
			target.Pix[i] = tv
			target.Pix[i+1] = tv
			target.Pix[i+2] = tv
		}
	}
	mi := (5*w + 5) * 3
	target.Pix[mi] = -1
	target.Pix[mi+1] = -1
	target.Pix[mi+2] = -1
	ts := 16
	ntx := (w + ts - 1) / ts
	nty := (h + ts - 1) / ts
	refTiles := make([]normStats, ntx*nty)
	for ty := 0; ty < nty; ty++ {
		for tx := 0; tx < ntx; tx++ {
			x0, y0 := tx*ts, ty*ts
			x1, y1 := x0+ts, y0+ts
			if x1 > w {
				x1 = w
			}
			if y1 > h {
				y1 = h
			}
			refTiles[ty*ntx+tx] = computeRegionStats(ref, x0, y0, x1, y1)
		}
	}
	refGlobal := computeNormStats(ref)
	targetN := localNormalizeWithRefTiles(target, refTiles, refGlobal, ntx, nty, ts)
	if targetN.Pix[mi] != -1 || targetN.Pix[mi+1] != -1 || targetN.Pix[mi+2] != -1 {
		return fmt.Errorf("masked pixel not preserved: %v", targetN.Pix[mi:mi+3])
	}
	return nil
}

func testIntrinsicQualityZeroStars() error {
	if got := computeIntrinsicQuality(0, 0, 0); got != 0 {
		return fmt.Errorf("zero-star quality = %v, want 0", got)
	}
	if got := computeIntrinsicQuality(50, 3.0, 0.01); got <= 0 {
		return fmt.Errorf("50-star quality = %v, want > 0", got)
	}
	return nil
}

func testFrameLimiterFlags() error {
	const refW, refH = 2400, 1600
	// Normal frame: no flags.
	normal := frameDiag{Quality: 80, BoxPct: 95, OffsetX: 5, OffsetY: -8}
	if flags := frameLimiterFlags(normal, refW, refH); len(flags) != 0 {
		return fmt.Errorf("normal frame got flags: %v", flags)
	}
	// M42-style 677 px horizontal drift on a 2400 px wide reference.
	drifted := frameDiag{Quality: 80, BoxPct: 95, OffsetX: -677, OffsetY: 10}
	flags := frameLimiterFlags(drifted, refW, refH)
	found := false
	for _, f := range flags {
		if f == "LARGE OFFSET" {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("677 px horizontal drift missing LARGE OFFSET: %v", flags)
	}
	// Vertical drift > 10% of refH.
	vertical := frameDiag{Quality: 80, BoxPct: 95, OffsetX: 0, OffsetY: 200}
	flags = frameLimiterFlags(vertical, refW, refH)
	found = false
	for _, f := range flags {
		if f == "LARGE OFFSET" {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("200 px vertical drift missing LARGE OFFSET: %v", flags)
	}
	// Under threshold must not fire.
	under := frameDiag{Quality: 80, BoxPct: 95, OffsetX: 220, OffsetY: 100}
	flags = frameLimiterFlags(under, refW, refH)
	for _, f := range flags {
		if f == "LARGE OFFSET" {
			return fmt.Errorf("under-threshold frame incorrectly flagged: %v", flags)
		}
	}
	// LOW QUALITY still works.
	lowQ := frameDiag{Quality: 20, BoxPct: 95, OffsetX: 0, OffsetY: 0}
	flags = frameLimiterFlags(lowQ, refW, refH)
	hasLowQ := false
	for _, f := range flags {
		if f == "LOW QUALITY" {
			hasLowQ = true
		}
	}
	if !hasLowQ {
		return fmt.Errorf("LOW QUALITY flag missing: %v", flags)
	}
	// LOW BOX%% still works.
	lowBox := frameDiag{Quality: 80, BoxPct: 40, OffsetX: 0, OffsetY: 0}
	flags = frameLimiterFlags(lowBox, refW, refH)
	hasLowBox := false
	for _, f := range flags {
		if f == "LOW BOX%" {
			hasLowBox = true
		}
	}
	if !hasLowBox {
		return fmt.Errorf("LOW BOX%% flag missing: %v", flags)
	}
	return nil
}

func testFrameLimiterFlagsRotationAware() error {
	// Case 1: exact Polaris IMG_7272.JPG — rot=25.77°, scale=1.0007,
	// offset=(440.2, -415.6) on 2256×1504. Legitimately rotated
	// (74-95 inliers); the old rotation-blind code fired LARGE
	// OFFSET here. Must NOT be flagged.
	polaris := frameDiag{
		Quality: 80, BoxPct: 95,
		Scale:       1.0007,
		RotationDeg: 25.77,
		OffsetX:     440.2,
		OffsetY:     -415.6,
	}
	if flags := frameLimiterFlags(polaris, 2256, 1504); len(flags) != 0 {
		return fmt.Errorf("Polaris IMG_7272 should not be flagged, got: %v", flags)
	}

	// Case 2: same rotation, plus 300 px of added horizontal drift.
	// Must fire LARGE OFFSET.
	drifted := frameDiag{
		Quality: 80, BoxPct: 95,
		Scale:       1.0007,
		RotationDeg: 25.77,
		OffsetX:     740.0,
		OffsetY:     -415.6,
	}
	flags := frameLimiterFlags(drifted, 2256, 1504)
	found := false
	for _, f := range flags {
		if f == "LARGE OFFSET" {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("drifted-rotated frame missing LARGE OFFSET: %v", flags)
	}

	// Case 3: M42-style 677 px horizontal drift on a 2400 px wide
	// reference (rotation 0, so the fix must not suppress it).
	m42 := frameDiag{
		Quality: 80, BoxPct: 95,
		Scale:       1.0,
		RotationDeg: 0.0,
		OffsetX:     677.0,
		OffsetY:     10.0,
	}
	flags = frameLimiterFlags(m42, 2400, 1600)
	found = false
	for _, f := range flags {
		if f == "LARGE OFFSET" {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("M42 drift missing LARGE OFFSET: %v", flags)
	}

	// Case 4: under-threshold, no rotation. Must NOT fire.
	under := frameDiag{
		Quality: 80, BoxPct: 95,
		Scale:       1.0,
		RotationDeg: 0.0,
		OffsetX:     220.0,
		OffsetY:     100.0,
	}
	flags = frameLimiterFlags(under, 2400, 1600)
	for _, f := range flags {
		if f == "LARGE OFFSET" {
			return fmt.Errorf("under-threshold frame incorrectly flagged: %v", flags)
		}
	}
	return nil
}

func testStarCacheRoundTrip() error {
	dir, err := os.MkdirTemp("", "skystacker-starcache-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	sc := starCache{
		Version:      cacheVersion,
		FrameMtime:   12345,
		FrameSize:    67890,
		DetectionSig: "max=100,scale=1",
		Stars:        []star{{X: 1.5, Y: 2.5, Flux: 100.0, Size: 3}, {X: 10, Y: 20, Flux: 50, Size: 4}},
		MedianBlob:   3.5,
		BgNoise:      0.02,
	}
	path := filepath.Join(dir, "test.json")
	if err := writeJSON(path, sc); err != nil {
		return err
	}
	loaded, ok := loadStarCache(path, "max=100,scale=1")
	if !ok {
		return errors.New("load failed")
	}
	if loaded.MedianBlob != 3.5 {
		return fmt.Errorf("median = %v, want 3.5", loaded.MedianBlob)
	}
	if len(loaded.Stars) != 2 {
		return fmt.Errorf("stars = %d, want 2", len(loaded.Stars))
	}
	if loaded.Stars[0].X != 1.5 {
		return fmt.Errorf("star[0].X = %v, want 1.5", loaded.Stars[0].X)
	}
	return nil
}

func testStarCacheInvalidation() error {
	dir, err := os.MkdirTemp("", "skystacker-starcache-inv-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "test.json")
	sc := starCache{Version: 0, DetectionSig: "x", Stars: []star{{X: 1, Y: 2, Flux: 3, Size: 4}}}
	if err := writeJSON(path, sc); err != nil {
		return err
	}
	if _, ok := loadStarCache(path, "x"); ok {
		return errors.New("expected version-0 cache to be rejected")
	}
	sc.Version = cacheVersion
	if err := writeJSON(path, sc); err != nil {
		return err
	}
	if _, ok := loadStarCache(path, "different-sig"); ok {
		return errors.New("expected sig mismatch to be rejected")
	}
	if _, ok := loadStarCache(path, "x"); !ok {
		return errors.New("expected matching cache to be accepted")
	}
	return nil
}

func testXfCacheInvalidation() error {
	dir, err := os.MkdirTemp("", "skystacker-xfcache-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	xc := xfCache{
		Version:      cacheVersion,
		RefMtime:     12345,
		HasTransform: true,
		Sim:          similarity{SC: 1, SS: 0, Tx: 5, Ty: 3},
		Inliers:      50,
	}
	path := filepath.Join(dir, "test.json")
	if err := writeJSON(path, xc); err != nil {
		return err
	}
	loaded, ok := loadXfCache(path, 12345)
	if !ok {
		return errors.New("expected hit with matching mtime")
	}
	if loaded.Inliers != 50 {
		return fmt.Errorf("inliers = %d, want 50", loaded.Inliers)
	}
	if _, ok := loadXfCache(path, 99999); ok {
		return errors.New("expected miss with mismatched mtime")
	}
	return nil
}

func testReportContains() error {
	dir, err := os.MkdirTemp("", "skystacker-report-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	outDir := filepath.Join(dir, "results")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	f := &frame{Width: 4, Height: 4, Pix: make([]float32, 4*4*3)}
	for i := range f.Pix {
		f.Pix[i] = 0.5
	}
	os.MkdirAll(filepath.Join(outDir, "TIFF"), 0o755)
	if err := writeFileAtomic(filepath.Join(outDir, "TIFF", "stacked_mean.tiff"),
		func(w io.Writer) error { return writeFloat32RGB(w, 4, 4, f.Pix) }); err != nil {
		return err
	}

	rep := &processingReport{
		Version: version, Timestamp: time.Now(),
		InputDir: "in", OutputDir: "out",
		Width: 4, Height: 4, Channels: 3,
		Method: "test", AccumulatorType: "float32",
		OutputFormat: "TIFF", OutputScaling: "none",
		RegistrationOn: false,
		Notes: []string{
			"No star registration or calibration was performed.",
			"JPEG inputs are gamma-encoded.",
		},
	}
	var buf bytes.Buffer
	if err := rep.writeTo(&buf); err != nil {
		return err
	}
	out := buf.String()
	for _, want := range []string{
		"SkyStacker", "Registration:", "Notes:",
		"No star registration or calibration",
	} {
		if !strings.Contains(out, want) {
			return fmt.Errorf("report missing %q", want)
		}
	}
	return nil
}

// =========================================================================
// testWriteGrayscaleMapsSuffix: the suffix parameter must select distinct
// filenames for the registered and unregistered rejection/weight maps, so
// that a registered sigma run keeps both. Earlier code dropped the
// unregistered maps silently when registration succeeded.
func testWriteGrayscaleMapsSuffix() error {
	dir, err := os.MkdirTemp("", "skystacker-maps-suffix-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	const w, h = 8, 8
	rej := make([]float32, w*h)
	wgt := make([]float32, w*h)
	for i := range rej {
		rej[i] = float32(i%5) / 5.0
		wgt[i] = float32(i%3) / 3.0
	}
	writeGrayscaleMaps(dir, w, h, rej, wgt, "")
	for _, name := range []string{"stacked_rejection_map.jpg", "stacked_weight_map.jpg"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("missing %s: %w", name, err)
		}
	}
	writeGrayscaleMaps(dir, w, h, rej, wgt, "_unregistered")
	for _, name := range []string{
		"stacked_unregistered_rejection_map.jpg",
		"stacked_unregistered_weight_map.jpg",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("missing %s: %w", name, err)
		}
	}
	writeGrayscaleMaps(dir, w, h, nil, nil, "")
	return nil
}

// testWriteCoverageMap: coverage map must be written for valid inputs,
// must use linear mapping (so 0 is always black and 1 is always white),
// and must no-op safely on nil / undersized / zero-total input.
func testWriteCoverageMap() error {
	dir, err := os.MkdirTemp("", "skystacker-coverage-map-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	const w, h = 8, 8
	counts := make([]int32, w*h)
	for i := range counts {
		counts[i] = 10
	}
	writeCoverageMap(dir, w, h, counts, 10, "")
	if _, err := os.Stat(filepath.Join(dir, "stacked_coverage_map.jpg")); err != nil {
		return fmt.Errorf("missing coverage map: %w", err)
	}
	counts[0] = 5
	writeCoverageMap(dir, w, h, counts, 10, "_unregistered")
	if _, err := os.Stat(filepath.Join(dir, "stacked_unregistered_coverage_map.jpg")); err != nil {
		return fmt.Errorf("missing suffixed coverage map: %w", err)
	}
	writeCoverageMap(dir, w, h, nil, 10, "")
	writeCoverageMap(dir, 16, 16, counts, 10, "")
	writeCoverageMap(dir, w, h, counts, 0, "")
	return nil
}

// testDetectCalibrationSubdirs verifies the auto-detection of the
// standard calibration folder layout. The feature exists so that
// `skystacker <dataset>/` discovers Light/, Dark/, Flat/ and Bias/
// subfolders without the user needing --calib-root or four separate
// flags.
func testDetectCalibrationSubdirs() error {
	// Case 1: empty directory.
	dir, err := os.MkdirTemp("", "skystacker-calib-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if got := detectCalibrationSubdirs(dir); got.Light != "" || got.Dark != "" || got.Flat != "" || got.Bias != "" {
		return fmt.Errorf("empty dir: got non-empty layout %+v", got)
	}

	// Case 2: canonical singular names.
	for _, name := range []string{"Light", "Dark", "Flat", "Bias"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			return err
		}
	}
	got := detectCalibrationSubdirs(dir)
	for name, path := range map[string]string{
		"Light": got.Light, "Dark": got.Dark,
		"Flat": got.Flat, "Bias": got.Bias,
	} {
		want := filepath.Join(dir, name)
		if path != want {
			return fmt.Errorf("%s: got %q, want %q", name, path, want)
		}
	}

	// Case 3: plural forms when singular is absent.
	dir2, err := os.MkdirTemp("", "skystacker-calib2-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir2)
	for _, name := range []string{"Lights", "Darks", "Flats", "Biases"} {
		if err := os.Mkdir(filepath.Join(dir2, name), 0o755); err != nil {
			return err
		}
	}
	got = detectCalibrationSubdirs(dir2)
	for name, want := range map[string]string{
		"Lights": got.Light, "Darks": got.Dark,
		"Flats": got.Flat, "Biases": got.Bias,
	} {
		if want != filepath.Join(dir2, name) {
			return fmt.Errorf("%s: got %q, want %q", name, want, filepath.Join(dir2, name))
		}
	}

	// Case 4: singular takes precedence over plural when both exist.
	dir3, err := os.MkdirTemp("", "skystacker-calib3-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir3)
	for _, name := range []string{"Light", "Lights"} {
		if err := os.Mkdir(filepath.Join(dir3, name), 0o755); err != nil {
			return err
		}
	}
	got = detectCalibrationSubdirs(dir3)
	if got.Light != filepath.Join(dir3, "Light") {
		return fmt.Errorf("singular preference: got %q, want Light/", got.Light)
	}

	// Case 5: exact folder name, not substring.
	dir4, err := os.MkdirTemp("", "skystacker-calib4-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir4)
	for _, name := range []string{"Lighter", "Darker", "Flatten"} {
		if err := os.Mkdir(filepath.Join(dir4, name), 0o755); err != nil {
			return err
		}
	}
	got = detectCalibrationSubdirs(dir4)
	if got.Light != "" || got.Dark != "" || got.Flat != "" || got.Bias != "" {
		return fmt.Errorf("substring names matched: %+v", got)
	}

	return nil
}

// =========================================================================
// testReportShowsWeightColumns: the report's per-frame diagnostic table
// must expose IntrQ (intrinsic quality) and Weight (combined weight used
// by the sigma reducer). Without this column the user cannot see which
// frames were down-weighted, violating the project's transparency goal.
func testReportShowsWeightColumns() error {
	rep := &processingReport{
		Version: version, Timestamp: time.Now(),
		InputDir: "in", OutputDir: "out",
		Width: 10, Height: 10, Channels: 3,
		Method: "sigma", AccumulatorType: "float32",
		OutputFormat: "TIFF", OutputScaling: "none",
		RegistrationOn: true,
		FrameDiags: []frameDiag{
			{
				Path:  "/tmp/frame_001.jpg",
				Stars: 100, Inliers: 80, Scale: 1.0, RotationDeg: 0.0,
				OffsetX: 0.5, OffsetY: 0.5,
				Quality: 80, BoxPct: 95,
				BgNoise: 0.02, MedianBlob: 4.5,
				IntrinsicQ: 75, Weight: 0.6,
			},
		},
	}
	var buf bytes.Buffer
	if err := rep.writeTo(&buf); err != nil {
		return err
	}
	out := buf.String()
	for _, want := range []string{"IntrQ", "Weight", "75.0", "0.60"} {
		if !strings.Contains(out, want) {
			return fmt.Errorf("report missing %q", want)
		}
	}
	return nil
}

// POST-PROCESSING PIPELINE (v0.4.0)
// =========================================================================
//
// Reads stacked_mean.tiff and produces:
//   post_step1_background_removed.jpg
//   post_step2_contrast_stretched.jpg
//   post_step3_color_calibrated.jpg
//   post_step4_sharpened.jpg
//   stacked_processed.tiff
//
// Each step is independent and reversible in isolation — you can inspect
// the intermediate JPEGs to see exactly what each stage contributes.

// --- Polynomial helpers (used by background removal) ---

func nCoeff(degree int) int { return (degree + 1) * (degree + 2) / 2 }

// polyBasis returns the monomial basis vector for a 2D polynomial of the
// given degree. Ordering: 1, x, y, x², xy, y², x³, x²y, xy², y³.
func polyBasis(x, y float64, degree int) []float64 {
	switch degree {
	case 1:
		return []float64{1, x, y}
	case 2:
		return []float64{1, x, y, x * x, x * y, y * y}
	case 3:
		return []float64{1, x, y, x * x, x * y, y * y, x * x * x, x * x * y, x * y * y, y * y * y}
	}
	return []float64{1}
}

func evalPoly(coeffs []float64, x, y float64, degree int) float64 {
	b := polyBasis(x, y, degree)
	var sum float64
	for i := range b {
		if i < len(coeffs) {
			sum += coeffs[i] * b[i]
		}
	}
	return sum
}

// solveLinear solves A·x = b for x via Gauss-Jordan with partial pivoting.
// Returns zeros if the matrix is singular.
func solveLinear(A [][]float64, b []float64, n int) []float64 {
	M := make([][]float64, n)
	for i := 0; i < n; i++ {
		M[i] = make([]float64, n+1)
		copy(M[i], A[i])
		M[i][n] = b[i]
	}
	for col := 0; col < n; col++ {
		pivot := col
		for row := col + 1; row < n; row++ {
			if math.Abs(M[row][col]) > math.Abs(M[pivot][col]) {
				pivot = row
			}
		}
		if math.Abs(M[pivot][col]) < 1e-12 {
			return make([]float64, n)
		}
		M[col], M[pivot] = M[pivot], M[col]
		pv := M[col][col]
		for j := col; j <= n; j++ {
			M[col][j] /= pv
		}
		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			f := M[row][col]
			if f == 0 {
				continue
			}
			for j := col; j <= n; j++ {
				M[row][j] -= f * M[col][j]
			}
		}
	}
	x := make([]float64, n)
	for i := 0; i < n; i++ {
		x[i] = M[i][n]
	}
	return x
}

func percentile(sorted []float32, pct float64) float32 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(pct * float64(len(sorted)-1) / 100)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// --- Step 1: background gradient removal ---

type bgSample struct {
	x, y float64    // normalised [-1, 1]
	v    [3]float64 // per-channel background estimate
}

func removeBackground(f *frame, degree int, useMask bool) *frame {
	var mask []bool
	if useMask {
		mask = make([]bool, f.Width*f.Height)
		for i := 0; i < f.Width*f.Height; i++ {
			lum := 0.299*f.Pix[i*3] + 0.587*f.Pix[i*3+1] + 0.114*f.Pix[i*3+2]
			if lum > 0.1 {
				mask[i] = true
			}
		}
	}

	if degree < 1 || degree > 3 {
		degree = 2
	}
	blockSize := 32
	if f.Width/8 < blockSize {
		blockSize = f.Width / 8
	}
	if f.Height/8 < blockSize {
		blockSize = f.Height / 8
	}
	if blockSize < 8 {
		blockSize = 8
	}
	nbx := (f.Width + blockSize - 1) / blockSize
	nby := (f.Height + blockSize - 1) / blockSize

	var samples []bgSample
	const subsample = 4
	for by := 0; by < nby; by++ {
		for bx := 0; bx < nbx; bx++ {
			x0 := bx * blockSize
			y0 := by * blockSize
			x1 := x0 + blockSize
			y1 := y0 + blockSize
			if x1 > f.Width {
				x1 = f.Width
			}
			if y1 > f.Height {
				y1 = f.Height
			}
			var rV, gV, bV []float32
			for y := y0; y < y1; y += subsample {
				for x := x0; x < x1; x += subsample {
					if useMask && mask[y*f.Width+x] {
						continue
					}
					i := (y*f.Width + x) * 3
					rV = append(rV, f.Pix[i])
					gV = append(gV, f.Pix[i+1])
					bV = append(bV, f.Pix[i+2])
				}
			}
			if len(rV) < 8 {
				continue
			}
			sort.Slice(rV, func(i, j int) bool { return rV[i] < rV[j] })
			sort.Slice(gV, func(i, j int) bool { return gV[i] < gV[j] })
			sort.Slice(bV, func(i, j int) bool { return bV[i] < bV[j] })
			cx := float64(x0+x1)/2.0/float64(f.Width)*2.0 - 1.0
			cy := float64(y0+y1)/2.0/float64(f.Height)*2.0 - 1.0
			samples = append(samples, bgSample{
				x: cx,
				y: cy,
				v: [3]float64{
					float64(percentile(rV, 20)),
					float64(percentile(gV, 20)),
					float64(percentile(bV, 20)),
				},
			})
		}
	}
	if len(samples) < nCoeff(degree)*2 {
		return f // not enough blocks — skip step
	}

	// Robust polynomial fit per channel
	coeffs := make([][]float64, 3)
	for c := 0; c < 3; c++ {
		active := make([]bool, len(samples))
		for i := range active {
			active[i] = true
		}
		n := nCoeff(degree)
		const iters = 3
		for iter := 0; iter < iters; iter++ {
			A := make([][]float64, n)
			for i := range A {
				A[i] = make([]float64, n)
			}
			b := make([]float64, n)
			count := 0
			for i := range samples {
				if !active[i] {
					continue
				}
				basis := polyBasis(samples[i].x, samples[i].y, degree)
				for j := 0; j < n; j++ {
					for k := 0; k < n; k++ {
						A[j][k] += basis[j] * basis[k]
					}
					b[j] += basis[j] * samples[i].v[c]
				}
				count++
			}
			if count < n {
				coeffs[c] = make([]float64, n)
				break
			}
			coeffs[c] = solveLinear(A, b, n)
			if iter < iters-1 {
				var residuals []float64
				for i := range samples {
					if !active[i] {
						continue
					}
					pred := evalPoly(coeffs[c], samples[i].x, samples[i].y, degree)
					residuals = append(residuals, samples[i].v[c]-pred)
				}
				var mean, sq float64
				for _, r := range residuals {
					mean += r
				}
				mean /= float64(len(residuals))
				for _, r := range residuals {
					sq += (r - mean) * (r - mean)
				}
				sigma := math.Sqrt(sq / float64(len(residuals)))
				if sigma < 1e-9 {
					sigma = 1e-9
				}
				// Reject samples that are significantly BRIGHTER than the
				// fit (those are nebulae/stars, not background).
				kept := 0
				for i := range samples {
					if !active[i] {
						continue
					}
					pred := evalPoly(coeffs[c], samples[i].x, samples[i].y, degree)
					if samples[i].v[c]-pred > 2.0*sigma {
						active[i] = false
					} else {
						kept++
					}
				}
				if kept < n {
					break
				}
			}
		}
	}

	// Apply: subtract the fitted surface
	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	invW := 2.0 / float64(f.Width-1)
	invH := 2.0 / float64(f.Height-1)
	for y := 0; y < f.Height; y++ {
		ny := float64(y)*invH - 1.0
		for x := 0; x < f.Width; x++ {
			nx := float64(x)*invW - 1.0
			i := (y*f.Width + x) * 3
			for c := 0; c < 3; c++ {
				bg := evalPoly(coeffs[c], nx, ny, degree)
				result.Pix[i+c] = f.Pix[i+c] - float32(bg)
			}
		}
	}
	return result
}

// --- Step 2: contrast stretch (asinh) ---

func stretchContrast(f *frame, level int) *frame {
	var lums []float32
	for i := 0; i < len(f.Pix); i += 3 {
		lums = append(lums, 0.299*f.Pix[i]+0.587*f.Pix[i+1]+0.114*f.Pix[i+2])
	}
	sort.Slice(lums, func(i, j int) bool { return lums[i] < lums[j] })

	pWhite := float64(lums[int(float64(len(lums)-1)*0.999)])
	if pWhite < 1e-5 {
		pWhite = 1.0
	}

	var D, SP float64
	switch level {
	case 1:
		D, SP = 0.5, 0.25 // Gentle
	case 2:
		D, SP = 1.5, 0.15 // Moderate
	case 3:
		D, SP = 3.0, 0.10 // Full
	}

	ghs := func(x float64) float64 {
		if x <= 0 {
			return 0
		}
		if x >= 0.9999 {
			return 1.0
		}
		num := (1 + D) * SP * x
		den := x + D*SP
		if den == 0 {
			return 0
		}
		return num / den
	}

	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for i := 0; i < len(f.Pix); i += 3 {
		r := float64(f.Pix[i])
		g := float64(f.Pix[i+1])
		b := float64(f.Pix[i+2])
		lum := 0.299*r + 0.587*g + 0.114*b
		if lum < 1e-6 {
			continue
		}
		xNorm := lum / pWhite
		newLum := ghs(xNorm)
		scale := newLum / lum
		result.Pix[i] = float32(math.Min(1.0, r*scale))
		result.Pix[i+1] = float32(math.Min(1.0, g*scale))
		result.Pix[i+2] = float32(math.Min(1.0, b*scale))
	}
	return result
}

// --- Step 3: color calibration / background neutralisation ---

func suppressChromaticAberration(f *frame) *frame {
	out := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for i := 0; i < len(f.Pix); i += 3 {
		r, g, b := f.Pix[i], f.Pix[i+1], f.Pix[i+2]
		excessR := r - g
		excessB := b - g
		// If both Red and Blue are significantly higher than Green,
		// it is a violet/magenta halo from an achromatic telescope.
		if excessR > 0.02 && excessB > 0.02 {
			out.Pix[i] = g + excessR*0.2
			out.Pix[i+1] = g
			out.Pix[i+2] = g + excessB*0.2
		} else {
			out.Pix[i] = r
			out.Pix[i+1] = g
			out.Pix[i+2] = b
		}
	}
	return out
}

func flattenBackground(f *frame, degree int, useMask bool) *frame {
	bg := removeBackground(f, degree, useMask)
	const pedestal = 0.02
	for i := range bg.Pix {
		bg.Pix[i] += pedestal
		if bg.Pix[i] > 1 {
			bg.Pix[i] = 1
		}
	}
	return bg
}

func calibrateColor(f *frame) *frame {
	var lums []float32
	const step = 4
	for y := 0; y < f.Height; y += step {
		for x := 0; x < f.Width; x += step {
			i := (y*f.Width + x) * 3
			lums = append(lums, 0.299*f.Pix[i]+0.587*f.Pix[i+1]+0.114*f.Pix[i+2])
		}
	}
	sort.Slice(lums, func(i, j int) bool { return lums[i] < lums[j] })

	// Use the mid-tones (10th to 50th percentile) to find the true sky background color.
	// The darkest 10% might be completely black due to background subtraction.
	pLow := percentile(lums, 10)
	pHigh := percentile(lums, 50)

	var rSamples, gSamples, bSamples []float32
	for i := 0; i < len(f.Pix); i += 3 {
		l := 0.299*f.Pix[i] + 0.587*f.Pix[i+1] + 0.114*f.Pix[i+2]
		if l >= pLow && l <= pHigh {
			rSamples = append(rSamples, f.Pix[i])
			gSamples = append(gSamples, f.Pix[i+1])
			bSamples = append(bSamples, f.Pix[i+2])
		}
	}
	if len(rSamples) == 0 {
		return f
	}
	sort.Slice(rSamples, func(i, j int) bool { return rSamples[i] < rSamples[j] })
	sort.Slice(gSamples, func(i, j int) bool { return gSamples[i] < gSamples[j] })
	sort.Slice(bSamples, func(i, j int) bool { return bSamples[i] < bSamples[j] })

	medR := float64(rSamples[len(rSamples)/2])
	medG := float64(gSamples[len(gSamples)/2])
	medB := float64(bSamples[len(bSamples)/2])

	// Prevent division by zero for very dark channels
	const eps = 1e-3
	if medR < eps {
		medR = eps
	}
	if medG < eps {
		medG = eps
	}
	if medB < eps {
		medB = eps
	}

	target := (medR + medG + medB) / 3.0
	// Clamp scales to [0.5, 1.5] to prevent extreme color shifts
	scaleR := clampFloat(target/medR, 0.5, 1.5)
	scaleG := clampFloat(target/medG, 0.5, 1.5)
	scaleB := clampFloat(target/medB, 0.5, 1.5)

	logf("  color calibration: medians R=%.3f G=%.3f B=%.3f -> scales R=%.3f G=%.3f B=%.3f",
		medR, medG, medB, scaleR, scaleG, scaleB)

	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for i := 0; i < len(f.Pix); i += 3 {
		r := float64(f.Pix[i]) * scaleR
		g := float64(f.Pix[i+1]) * scaleG
		b := float64(f.Pix[i+2]) * scaleB
		if r > 1 {
			r = 1
		}
		if g > 1 {
			g = 1
		}
		if b > 1 {
			b = 1
		}
		if r < 0 {
			r = 0
		}
		if g < 0 {
			g = 0
		}
		if b < 0 {
			b = 0
		}
		result.Pix[i] = float32(r)
		result.Pix[i+1] = float32(g)
		result.Pix[i+2] = float32(b)
	}
	return result
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// --- Step 4: unsharp mask ---

func gaussianBlur(f *frame, sigma float64) *frame {
	if sigma <= 0 {
		sigma = 1.5
	}
	radius := int(math.Ceil(sigma * 3))
	if radius < 1 {
		radius = 1
	}
	kernelSize := radius*2 + 1
	kernel := make([]float32, kernelSize)
	var sum float32
	for i := 0; i < kernelSize; i++ {
		off := float64(i - radius)
		kernel[i] = float32(math.Exp(-off * off / (2 * sigma * sigma)))
		sum += kernel[i]
	}
	for i := range kernel {
		kernel[i] /= sum
	}
	// Horizontal pass
	tmp := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			for c := 0; c < 3; c++ {
				var acc float32
				for k := 0; k < kernelSize; k++ {
					sx := x + k - radius
					if sx < 0 {
						sx = 0
					} else if sx >= f.Width {
						sx = f.Width - 1
					}
					acc += kernel[k] * f.Pix[(y*f.Width+sx)*3+c]
				}
				tmp.Pix[(y*f.Width+x)*3+c] = acc
			}
		}
	}
	// Vertical pass
	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			for c := 0; c < 3; c++ {
				var acc float32
				for k := 0; k < kernelSize; k++ {
					sy := y + k - radius
					if sy < 0 {
						sy = 0
					} else if sy >= f.Height {
						sy = f.Height - 1
					}
					acc += kernel[k] * tmp.Pix[(sy*f.Width+x)*3+c]
				}
				result.Pix[(y*f.Width+x)*3+c] = acc
			}
		}
	}
	return result
}

func unsharpMask(f *frame, amount, radius float64) *frame {
	blurred := gaussianBlur(f, radius)
	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	amt := float32(amount)
	for i := range f.Pix {
		o := f.Pix[i]
		b := blurred.Pix[i]
		v := o + amt*(o-b)
		if v < 0 {
			v = 0
		} else if v > 1 {
			v = 1
		}
		result.Pix[i] = v
	}
	return result
}

// --- Pipeline driver ---

func applyHDR(f *frame, amount float64) *frame {
	if amount <= 0 {
		return f
	}
	// Use a large blur for Local Contrast Enhancement (LCE)
	blurred := gaussianBlur(f, 30.0)
	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for i := range f.Pix {
		v := float64(f.Pix[i]) + (float64(f.Pix[i])-float64(blurred.Pix[i]))*amount
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		result.Pix[i] = float32(v)
	}
	// Boost saturation for vivid colors
	for i := 0; i < len(result.Pix); i += 3 {
		r, g, b := result.Pix[i], result.Pix[i+1], result.Pix[i+2]
		lum := 0.299*r + 0.587*g + 0.114*b
		r = lum + (r-lum)*1.2
		g = lum + (g-lum)*1.2
		b = lum + (b-lum)*1.2
		if r > 1 {
			r = 1
		}
		if g > 1 {
			g = 1
		}
		if b > 1 {
			b = 1
		}
		if r < 0 {
			r = 0
		}
		if g < 0 {
			g = 0
		}
		if b < 0 {
			b = 0
		}
		result.Pix[i], result.Pix[i+1], result.Pix[i+2] = r, g, b
	}
	return result
}

// removeStars detects stars and replaces each with the median of an
// annulus around it. The star is completely erased, revealing the diffuse
// nebula/galaxy signal behind it.
func removeStars(f *frame, sigma float64, radius int) *frame {
	if radius <= 0 {
		radius = (f.Width + f.Height) / 400
		if radius < 8 {
			radius = 8
		}
	}
	stars := detectStarsSigma(f, 100000, sigma)
	logf("  star removal: %d stars detected (sigma=%.1f, maxRadius=%d)",
		len(stars), sigma, radius)
	if len(stars) >= 100000 {
		logf("  WARNING: hit 100000-star detection cap; some very faint stars may remain")
	}

	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	copy(result.Pix, f.Pix)

	for _, s := range stars {
		cx, cy := int(s.X+0.5), int(s.Y+0.5)

		starRadius := radius
		switch {
		case s.Size <= 2:
			starRadius = 2
		case s.Size <= 5:
			starRadius = radius / 2
			if starRadius < 3 {
				starRadius = 3
			}
		case s.Size <= 20:
			starRadius = (radius * 2) / 3
			if starRadius < 4 {
				starRadius = 4
			}
		}
		rInner := starRadius
		rOuter := starRadius * 2
		if rOuter > radius*2 {
			rOuter = radius * 2
		}

		var rS, gS, bS []float32
		for dy := -rOuter; dy <= rOuter; dy++ {
			for dx := -rOuter; dx <= rOuter; dx++ {
				d2 := dx*dx + dy*dy
				if d2 < rInner*rInner || d2 > rOuter*rOuter {
					continue
				}
				nx, ny := cx+dx, cy+dy
				if nx < 0 || ny < 0 || nx >= f.Width || ny >= f.Height {
					continue
				}
				i := (ny*f.Width + nx) * 3
				rS = append(rS, f.Pix[i])
				gS = append(gS, f.Pix[i+1])
				bS = append(bS, f.Pix[i+2])
			}
		}
		if len(rS) == 0 {
			continue
		}
		sort.Slice(rS, func(i, j int) bool { return rS[i] < rS[j] })
		sort.Slice(gS, func(i, j int) bool { return gS[i] < gS[j] })
		sort.Slice(bS, func(i, j int) bool { return bS[i] < bS[j] })
		rMed := rS[len(rS)/2]
		gMed := gS[len(gS)/2]
		bMed := bS[len(bS)/2]

		for dy := -starRadius; dy <= starRadius; dy++ {
			for dx := -starRadius; dx <= starRadius; dx++ {
				if dx*dx+dy*dy > starRadius*starRadius {
					continue
				}
				nx, ny := cx+dx, cy+dy
				if nx < 0 || ny < 0 || nx >= f.Width || ny >= f.Height {
					continue
				}
				i := (ny*f.Width + nx) * 3
				result.Pix[i] = rMed
				result.Pix[i+1] = gMed
				result.Pix[i+2] = bMed
			}
		}
	}
	return result
}

func reduceStars(f *frame, amount float64) *frame {
	if amount <= 0 {
		return f
	}
	const threshold = 0.15
	const fullStrengthAbove = 0.5
	const radius = 2
	out := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			i := (y*f.Width + x) * 3
			out.Pix[i], out.Pix[i+1], out.Pix[i+2] = f.Pix[i], f.Pix[i+1], f.Pix[i+2]
			if y < radius || y >= f.Height-radius || x < radius || x >= f.Width-radius {
				continue
			}
			r, g, b := f.Pix[i], f.Pix[i+1], f.Pix[i+2]
			lum := 0.299*r + 0.587*g + 0.114*b
			if lum <= threshold {
				continue
			}
			var minLum float32 = 1.0
			minR, minG, minB := r, g, b
			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					ni := ((y+dy)*f.Width + (x + dx)) * 3
					nLum := 0.299*f.Pix[ni] + 0.587*f.Pix[ni+1] + 0.114*f.Pix[ni+2]
					if nLum < minLum {
						minLum = nLum
						minR, minG, minB = f.Pix[ni], f.Pix[ni+1], f.Pix[ni+2]
					}
				}
			}
			strength := float32(amount)
			if lum < fullStrengthAbove {
				strength *= (lum - threshold) / (fullStrengthAbove - threshold)
			}
			out.Pix[i] = r*(1-strength) + minR*strength
			out.Pix[i+1] = g*(1-strength) + minG*strength
			out.Pix[i+2] = b*(1-strength) + minB*strength
		}
	}
	return out
}

// blackPointShift subtracts the given percentile of the luminance
// histogram from every channel and clamps to [0, 1]. This gives the darkest
// regions of the image a true-black floor, which visually reads as a "dark
// background" after stretch. A percentile of 0 disables the step.
func blackPointShift(f *frame, percentile float64) *frame {
	if percentile <= 0 || percentile >= 100 {
		return f
	}
	var lums []float32
	const step = 4
	for y := 0; y < f.Height; y += step {
		for x := 0; x < f.Width; x += step {
			i := (y*f.Width + x) * 3
			lums = append(lums, 0.299*f.Pix[i]+0.587*f.Pix[i+1]+0.114*f.Pix[i+2])
		}
	}
	if len(lums) == 0 {
		return f
	}
	sort.Slice(lums, func(i, j int) bool { return lums[i] < lums[j] })
	idx := int(float64(len(lums)-1) * percentile / 100)
	threshold := float64(lums[idx])

	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for i := range f.Pix {
		v := float64(f.Pix[i]) - threshold
		if v < 0 {
			v = 0
		} else if v > 1 {
			v = 1
		}
		result.Pix[i] = float32(v)
	}
	logf("  black point shift: subtract P%g = %.4f", percentile, threshold)
	return result
}

// mtf implements the Midtones Transfer Function used by PixInsight's STF.
// MTF(m, x) = ((m-1)*x) / ((2m-1)*x - m).  MTF(0.5, x) = x (identity).
func mtf(m, x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	num := (m - 1) * x
	den := (2*m-1)*x - m
	if den == 0 {
		return 0
	}
	v := num / den
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// computeSTFParams returns the shadow clip point (c0) and midtones balance
// (m) for one channel. shadowsClip is in sigma units (PixInsight default
// -2.8); targetBG is the desired background level after stretching.
func computeSTFParams(vals []float32, shadowsClip, targetBG float64) (c0, m float64) {
	if shadowsClip >= 0 {
		shadowsClip = -2.8
	}
	if shadowsClip < -10 {
		shadowsClip = -10
	}
	if targetBG <= 0 || targetBG >= 1 {
		targetBG = 0.25
	}
	if len(vals) == 0 {
		return 0, 0.5
	}
	step := 1
	if len(vals) > 200000 {
		step = len(vals) / 100000
		if step < 1 {
			step = 1
		}
	}
	sampled := make([]float32, 0, len(vals)/step+1)
	for i := 0; i < len(vals); i += step {
		sampled = append(sampled, vals[i])
	}
	sort.Slice(sampled, func(i, j int) bool { return sampled[i] < sampled[j] })
	n := len(sampled)
	if n == 0 {
		return 0, 0.5
	}
	median := float64(sampled[n/2])
	absDev := make([]float32, n)
	for i := 0; i < n; i++ {
		absDev[i] = float32(math.Abs(float64(sampled[i]) - median))
	}
	sort.Slice(absDev, func(i, j int) bool { return absDev[i] < absDev[j] })
	mad := float64(absDev[n/2])
	sigma := 1.4826 * mad
	c0 = median + shadowsClip*sigma
	if c0 < 0 {
		c0 = 0
	}
	if c0 > 0.99 {
		c0 = 0.99
	}
	medNorm := (median - c0) / (1 - c0)
	if medNorm <= 1e-4 {
		medNorm = 1e-4
	}
	if medNorm >= 0.999 {
		medNorm = 0.999
	}
	m = mtf(targetBG, medNorm)
	if math.IsNaN(m) || math.IsInf(m, 0) {
		m = 0.5
	}
	return c0, m
}

// autoStretch applies a PixInsight-style STF/MTF auto-stretch.  shadowsClip
// is in units of sigma (default -2.8); targetBG is the target background
// level (default 0.25).  linked=true shares a single transform across all
// three channels (preserves channel ratios); linked=false computes a
// separate transform per channel (better automatic colour balance).
func autoStretch(f *frame, shadowsClip, targetBG float64, linked bool) *frame {
	if shadowsClip >= 0 {
		shadowsClip = -2.8
	}
	if targetBG <= 0 || targetBG >= 1 {
		targetBG = 0.25
	}
	const nCh = 3
	type chanStats struct {
		c0, m float64
	}
	stats := [nCh]chanStats{}
	if linked {
		lums := make([]float32, 0, f.Width*f.Height)
		for i := 0; i < len(f.Pix); i += 3 {
			lums = append(lums, 0.299*f.Pix[i]+0.587*f.Pix[i+1]+0.114*f.Pix[i+2])
		}
		c0, m := computeSTFParams(lums, shadowsClip, targetBG)
		for c := 0; c < nCh; c++ {
			stats[c] = chanStats{c0: c0, m: m}
		}
		logf("  auto-stretch (linked): c0=%.4f m=%.4f", c0, m)
	} else {
		for c := 0; c < nCh; c++ {
			vals := make([]float32, 0, f.Width*f.Height)
			for i := c; i < len(f.Pix); i += 3 {
				vals = append(vals, f.Pix[i])
			}
			c0, m := computeSTFParams(vals, shadowsClip, targetBG)
			stats[c] = chanStats{c0: c0, m: m}
			logf("  auto-stretch: channel %d c0=%.4f m=%.4f", c, c0, m)
		}
	}
	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			i := (y*f.Width + x) * 3
			for c := 0; c < nCh; c++ {
				st := stats[c]
				v := float64(f.Pix[i+c])
				norm := (v - st.c0) / (1 - st.c0)
				out := mtf(st.m, norm)
				if math.IsNaN(out) || math.IsInf(out, 0) {
					out = 0
				}
				result.Pix[i+c] = float32(out)
			}
		}
	}
	return result
}

// blendHDR combines a stretched display frame (base, typically step 4)
// with a linear background-removed frame (detail, typically step 1) via
// pixel-wise addition, clamped to [0, 1]. The stretched base supplies the
// overall tone curve; the linear detail supplies bright-core information
// that the auto-stretch compressed toward white. factor controls how much
// of the detail frame is mixed in (1.0 = full addition, 0 = base only).
func blendHDR(base, detail *frame, factor float64) *frame {
	if base.Width != detail.Width || base.Height != detail.Height {
		return base
	}
	if factor <= 0 {
		return base
	}
	result := &frame{Width: base.Width, Height: base.Height, Pix: make([]float32, len(base.Pix))}
	f := float32(factor)
	for i := range base.Pix {
		d := detail.Pix[i]
		if d < 0 {
			d = 0
		}
		v := base.Pix[i] + f*d
		if v < 0 {
			v = 0
		} else if v > 1 {
			v = 1
		}
		result.Pix[i] = v
	}
	return result
}

// guidedFilterGray is a self-guided filter (He et al., 2010).
func guidedFilterGray(I []float32, w, h, radius int, eps float64) []float32 {
	n := w * h
	if n == 0 {
		return nil
	}
	if radius < 1 {
		radius = 1
	}
	if eps <= 0 {
		eps = 1e-4
	}
	mean_I := boxBlurLuminance(I, w, h, radius)
	II := make([]float32, n)
	for i := 0; i < n; i++ {
		II[i] = I[i] * I[i]
	}
	mean_II := boxBlurLuminance(II, w, h, radius)
	a := make([]float32, n)
	b := make([]float32, n)
	for i := 0; i < n; i++ {
		varI := float64(mean_II[i]) - float64(mean_I[i])*float64(mean_I[i])
		if varI < 0 {
			varI = 0
		}
		av := varI / (varI + eps)
		a[i] = float32(av)
		b[i] = mean_I[i] * float32(1-av)
	}
	mean_a := boxBlurLuminance(a, w, h, radius)
	mean_b := boxBlurLuminance(b, w, h, radius)
	q := make([]float32, n)
	for i := 0; i < n; i++ {
		q[i] = mean_a[i]*I[i] + mean_b[i]
	}
	return q
}

// guidedFilterFrameOnce applies the self-guided filter to every channel of f
// without any blending. The result IS the fully filtered frame; the caller
// decides how much of it to keep.
func guidedFilterFrameOnce(f *frame, radius int, eps float64) *frame {
	nPix := f.Width * f.Height
	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	I := make([]float32, nPix)
	for c := 0; c < 3; c++ {
		for i := 0; i < nPix; i++ {
			I[i] = f.Pix[i*3+c]
		}
		den := guidedFilterGray(I, f.Width, f.Height, radius, eps)
		for i := 0; i < nPix; i++ {
			v := den[i]
			if v < 0 {
				v = 0
			} else if v > 1 {
				v = 1
			}
			result.Pix[i*3+c] = v
		}
	}
	return result
}

// denoiseFrame reduces pixel noise in f by applying the self-guided filter
// per channel. strength controls how many passes are run: 1.0 is one full
// pass, 2.0 is two full passes, 2.5 is two full passes followed by a third
// blended at 50%. Values in (0, 1) do a single pass blended by the given
// fraction. Each additional full pass compounds the smoothing power of the
// filter, so strength=4.0 is meaningfully smoother than strength=1.0 without
// raising radius. Pass counts are capped at 10 by parseArgs.
func denoiseFrame(f *frame, radius int, eps, strength float64, chromaMul ...float64) *frame {
	if strength <= 0 {
		return f
	}
	if radius < 1 {
		radius = 3
	}
	nFull := int(math.Floor(strength))
	frac := float32(strength - float64(nFull))

	cur := f
	for i := 0; i < nFull; i++ {
		cur = guidedFilterFrameOnce(cur, radius, eps)
	}
	if frac > 1e-6 {
		filtered := guidedFilterFrameOnce(cur, radius, eps)
		out := &frame{Width: cur.Width, Height: cur.Height, Pix: make([]float32, len(cur.Pix))}
		for i := range cur.Pix {
			v := cur.Pix[i] + frac*(filtered.Pix[i]-cur.Pix[i])
			if v < 0 {
				v = 0
			} else if v > 1 {
				v = 1
			}
			out.Pix[i] = v
		}
		cur = out
	}

	// Chroma-selective coarse pass: removes low-frequency
	// "color blotch" noise (red/green/blue granularity 15-40 px
	// wide) that small-radius passes cannot see, without
	// softening the luminance detail that defines nebula
	// structure. Only fires for strength >= 1.5 so a very mild
	// --denoise-strength 1 does not silently gain a large
	// chroma blur.
	if strength >= 1.5 {
		cm := 1.0
		if len(chromaMul) > 0 {
			cm = chromaMul[0]
		}
		if cm > 0 {
			logf("  denoise: chroma multi-scale pass (%.2f full iterations, strength=%g)", cm, cm)
			cur = chromaDenoiseMultiScale(cur, cm)
		}
	}

	return cur
}

// blendWeighted produces a pixel-wise weighted lerp between a and b:
// out = wa*a + (1-wa)*b, clamped to [0, 1]. wa=1 returns a; wa=0 returns b.
// Used by the final step-7 outputs which present a spectrum from
// "mostly step 1 (background removed)" to "mostly step 6 (HDR)".
func blendWeighted(a, b *frame, wa float64) *frame {
	if a.Width != b.Width || a.Height != b.Height {
		return a
	}
	if wa <= 0 {
		return b
	}
	if wa >= 1 {
		return a
	}
	result := &frame{Width: a.Width, Height: a.Height, Pix: make([]float32, len(a.Pix))}
	waf := float32(wa)
	wbf := float32(1 - wa)
	for i := range a.Pix {
		v := waf*a.Pix[i] + wbf*b.Pix[i]
		if v < 0 {
			v = 0
		} else if v > 1 {
			v = 1
		}
		result.Pix[i] = v
	}
	return result
}

// chromaDenoiseMultiScale applies a two-scale chroma-only denoise that
// targets low-frequency "color blotch" noise without washing out real
// colour. It is the successor to chromaDenoiseGB for actual denoise use:
//
//   - It runs at TWO radii (24 px and 96 px, capped to min(W,H)/6 on
//     small images) so that blotches across the 20-150 px size range are
//     all smoothed. A single radius can only reach one octave.
//
//   - The chroma eps is 0.005, not 0.02. High eps makes the guided
//     filter average every neighbourhood toward its local mean, which is
//     exactly what caused the earlier greenish/cyan wash-out: real
//     colour edges got blurred into mud. Low eps preserves real colour
//     and only removes noise.
//
//   - Only chroma (Cb, Cr) is filtered. Luminance is left completely
//     untouched, so nebula structure stays sharp. A previous version
//     applied a mild lum pass; that contributed to the "washed out"
//     look and is removed here.
//
//   - strength is a straight multiplier on the two blend weights.
//     strength=1.0 uses (0.85, 0.45); strength=0 returns f unchanged.
//
// The two filtered chroma channels are recombined with the untouched
// luminance. No saturation boost is applied: the point of lowering eps
// is that real colour survives, so there is nothing to compensate for.
// chromaDenoiseMultiScaleOnce performs one full pass of the two-scale
// chroma filter: radius 24 (weight 0.85) and radius 96 (weight 0.55).
// blend is a straight multiplier on both weights and must be in (0, 1].
// Callers who want more power should iterate the whole pass with
// chromaDenoiseMultiScale.
func chromaDenoiseMultiScaleOnce(f *frame, blend float64) *frame {
	if blend <= 0 {
		return f
	}
	if blend > 1 {
		blend = 1
	}
	nPix := f.Width * f.Height
	Y := make([]float32, nPix)
	Cb := make([]float32, nPix)
	Cr := make([]float32, nPix)
	for i := 0; i < nPix; i++ {
		r := float64(f.Pix[i*3])
		g := float64(f.Pix[i*3+1])
		b := float64(f.Pix[i*3+2])
		y := 0.299*r + 0.587*g + 0.114*b
		Y[i] = float32(y)
		Cb[i] = float32(b - y)
		Cr[i] = float32(r - y)
	}
	maxR := f.Width
	if f.Height < maxR {
		maxR = f.Height
	}
	maxR /= 6
	if maxR < 16 {
		maxR = 16
	}
	scales := []struct {
		radius int
		weight float64
	}{
		{24, 0.85},
		{96, 0.55},
	}
	for _, s := range scales {
		r := s.radius
		if r > maxR {
			r = maxR
		}
		w := float32(s.weight * blend)
		if w <= 0 {
			continue
		}
		Cbd := guidedFilterGray(Cb, f.Width, f.Height, r, 0.005)
		Crd := guidedFilterGray(Cr, f.Width, f.Height, r, 0.005)
		for i := 0; i < nPix; i++ {
			Cb[i] = Cb[i] + w*(Cbd[i]-Cb[i])
			Cr[i] = Cr[i] + w*(Crd[i]-Cr[i])
		}
	}
	const cCrToG = 0.299 / 0.587
	const cCbToG = 0.114 / 0.587
	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for i := 0; i < nPix; i++ {
		y := Y[i]
		cr := Cr[i]
		cb := Cb[i]
		r := y + cr
		b := y + cb
		g := y - float32(cCrToG)*cr - float32(cCbToG)*cb
		if r < 0 {
			r = 0
		} else if r > 1 {
			r = 1
		}
		if g < 0 {
			g = 0
		} else if g > 1 {
			g = 1
		}
		if b < 0 {
			b = 0
		} else if b > 1 {
			b = 1
		}
		result.Pix[i*3] = r
		result.Pix[i*3+1] = g
		result.Pix[i*3+2] = b
	}
	return result
}

// chromaDenoiseMultiScale is the strength-aware wrapper around
// chromaDenoiseMultiScaleOnce. Fractional strengths below 1 do a single
// blended pass; integer strengths above 1 iterate the whole pass that
// many times. Each iteration re-runs both guided-filter scales on the
// previous iteration's output, so the smoothing power compounds instead
// of saturating at weight=1.0.
//
// strength=0.5 : one blended pass at 50%
// strength=1.0 : one full pass (radius 24 + radius 96)
// strength=2.0 : two full passes
// strength=4.0 : four full passes
// strength=10  : ten full passes (effectively flattens chroma)
//
// Capped at 10 iterations to keep runtime bounded (each full pass is
// roughly 10 s on a 2400x1600 image, so 10 passes ~ 100 s).
func chromaDenoiseMultiScale(f *frame, strength float64) *frame {
	if strength <= 0 {
		return f
	}
	nFull := int(math.Floor(strength))
	frac := strength - float64(nFull)
	if nFull > 10 {
		nFull = 10
	}
	cur := f
	for i := 0; i < nFull; i++ {
		cur = chromaDenoiseMultiScaleOnce(cur, 1.0)
	}
	if frac > 1e-6 {
		cur = chromaDenoiseMultiScaleOnce(cur, frac)
	}
	return cur
}

// chromaDenoiseGB is a chroma-selective denoise pass. It splits the
// image into approximate luminance (Y) and chroma (Cb, Cr), applies a
// wide, aggressive guided filter to chroma and a narrower, milder one
// to luminance, then recombines. This is the standard technique for
// removing low-frequency "color blotch" noise: the eye is far less
// sensitive to chroma blur than to luminance blur, so chroma can be
// smoothed much more aggressively without the image looking soft.
//
// Small-radius per-channel guided filters cannot fix these blobs
// because the blobs are 15-40 px wide and have similar local variance
// to real luminance structure, so the guided filter mistakes them for
// detail and preserves them.
//
// chromaStrength controls how much of the chroma filtered result is
// blended in (0 = no change, 1 = fully filtered).
// lumStrength controls the same for luminance, and is meant to be
// small (0.1-0.3) so that nebula detail survives.
func chromaDenoiseGB(f *frame, radius int, eps, chromaStrength, lumStrength float64) *frame {
	if chromaStrength <= 0 && lumStrength <= 0 {
		return f
	}
	if radius < 1 {
		radius = 24
	}
	nPix := f.Width * f.Height
	Y := make([]float32, nPix)
	Cb := make([]float32, nPix)
	Cr := make([]float32, nPix)
	for i := 0; i < nPix; i++ {
		r := float64(f.Pix[i*3])
		g := float64(f.Pix[i*3+1])
		b := float64(f.Pix[i*3+2])
		y := 0.299*r + 0.587*g + 0.114*b
		Y[i] = float32(y)
		Cb[i] = float32(b - y)
		Cr[i] = float32(r - y)
	}
	if chromaStrength > 0 {
		Cbd := guidedFilterGray(Cb, f.Width, f.Height, radius, eps)
		Crd := guidedFilterGray(Cr, f.Width, f.Height, radius, eps)
		cs := float32(chromaStrength)
		for i := 0; i < nPix; i++ {
			Cb[i] = Cb[i] + cs*(Cbd[i]-Cb[i])
			Cr[i] = Cr[i] + cs*(Crd[i]-Cr[i])
		}
	}
	if lumStrength > 0 {
		lumRadius := radius / 4
		if lumRadius < 3 {
			lumRadius = 3
		}
		Yd := guidedFilterGray(Y, f.Width, f.Height, lumRadius, eps*0.5)
		ls := float32(lumStrength)
		for i := 0; i < nPix; i++ {
			Y[i] = Y[i] + ls*(Yd[i]-Y[i])
		}
	}
	// Recombine RGB from Y, Cb, Cr.
	//   r = y + cr
	//   b = y + cb
	//   g = y - (0.299/0.587)*cr - (0.114/0.587)*cb
	const cCrToG = 0.299 / 0.587
	const cCbToG = 0.114 / 0.587
	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for i := 0; i < nPix; i++ {
		y := Y[i]
		cr := Cr[i]
		cb := Cb[i]
		r := y + cr
		b := y + cb
		g := y - float32(cCrToG)*cr - float32(cCbToG)*cb
		if r < 0 {
			r = 0
		} else if r > 1 {
			r = 1
		}
		if g < 0 {
			g = 0
		} else if g > 1 {
			g = 1
		}
		if b < 0 {
			b = 0
		} else if b > 1 {
			b = 1
		}
		result.Pix[i*3] = r
		result.Pix[i*3+1] = g
		result.Pix[i*3+2] = b
	}
	return result
}

// applyCurves applies a background-darkening curve to f. It crushes the
// deepest shadows (values below `black` become 0) and darkens the midtones
// (each channel value is raised to `gamma`). Highlights are preserved: a
// pixel at 1.0 stays at 1.0. Applied identically to each channel so hue is
// preserved.
//
// gamma > 1 darkens, gamma < 1 brightens. black > 0 crushes the dark tail
// to pure black. black = 0 and gamma = 1 returns f unchanged.
//
// The effect matches a GIMP Curves adjustment with a smooth curve that
// pulls the lower-mid part of the histogram down and leaves the top end
// alone: a deeper black floor and richer shadow contrast without losing
// bright stars or nebula highlights.
func applyCurves(f *frame, black, gamma float64) *frame {
	if black <= 0 && math.Abs(gamma-1.0) < 1e-6 {
		return f
	}
	if black < 0 {
		black = 0
	}
	if black >= 0.99 {
		black = 0.99
	}
	if gamma <= 0 {
		gamma = 1.0
	}
	b := float32(black)
	inv := float32(1.0 / (1.0 - black))
	g := gamma
	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	for i := range f.Pix {
		x := (f.Pix[i] - b) * inv
		if x <= 0 {
			result.Pix[i] = 0
			continue
		}
		if x >= 1 {
			result.Pix[i] = 1
			continue
		}
		if g == 1.0 {
			result.Pix[i] = x
		} else {
			result.Pix[i] = float32(math.Pow(float64(x), g))
		}
	}
	return result
}

// selectSpikeStars detects the brightest stars in f and returns the top
// `percent` percent of them, sorted by decreasing flux. Capped at 100
// stars (a practical limit on dense fields) and floored at 1 (so a very
// small percent still produces a visible effect on sparse fields). Returns
// nil if percent <= 0 or no stars are detected at all.
//
// Detection uses 5 sigma, the strictest threshold used elsewhere in the
// pipeline, so noise blobs never become spikes.
func selectSpikeStars(f *frame, percent float64, minCount int) []star {
	if percent <= 0 {
		return nil
	}
	stars := detectStarsSigma(f, 5000, 5.0)
	if len(stars) == 0 {
		return nil
	}
	n := int(float64(len(stars)) * percent / 100.0)
	if n < 1 {
		n = 1
	}
	if minCount > 0 && n < minCount {
		n = minCount
	}
	if n > 100 {
		n = 100
	}
	if n > len(stars) {
		n = len(stars)
	}
	return stars[:n]
}

// overlaySpikes draws diffraction spikes through each star's centroid.
// The intensity and length of each star's spikes scale with that star's
// flux relative to the brightest star in the list, following a
// power-law falloff controlled by `softness`:
//
//	weight = (flux / maxFlux) ^ softness
//
// softness = 1.0 is linear (a star half as bright gets a half-length
// spike). softness < 1 compresses the range, so mid-brightness stars
// get longer spikes than linear would give them — this avoids the
// "hard step from barely-visible to zero" that pure linear falloff
// produces. `minRatio` then acts as a floor so the faintest star in
// the list still gets a visible spike.
//
// The base angle determines the arm orientation (shared by every star
// in the frame, per the "same orientation for all stars" requirement);
// each arm is at baseAngle + arm * 2*pi/numArms.
//
// The effect is purely additive: spikes brighten pixels along their rays
// but never darken anything, so applying it on top of a finished
// post-processing pipeline cannot destroy background or star detail.
//
// Parameters:
//   - f:             input frame
//   - stars:         star list; the first entry is used as the brightness
//     reference. Typically the output of selectSpikeStars.
//   - baseLength:         arm half-length in pixels for the brightest star
//   - baseIntensity:      peak additive brightness for the brightest star
//   - numArms:            number of spike arms (2-12, clamped)
//   - baseAngle:          base orientation in radians
//   - intensitySoftness:  exponent on relFlux for spike brightness
//   - intensityMinRatio:  floor for spike brightness
//   - lengthSoftness:     exponent on relFlux for arm length
//   - lengthMinRatio:     floor for arm length
//
// Returns a new frame. Returns f unchanged if stars is empty, any of
// baseLength/baseIntensity is non-positive, or numArms < 2.
func overlaySpikes(f *frame, stars []star, baseLength, baseIntensity float64, numArms int, baseAngle, intensitySoftness, intensityMinRatio, lengthSoftness, lengthMinRatio float64) *frame {
	if len(stars) == 0 || baseLength <= 0 || baseIntensity <= 0 || numArms < 2 {
		return f
	}
	if numArms > 12 {
		numArms = 12
	}
	maxFlux := stars[0].Flux
	if maxFlux <= 0 {
		return f
	}
	// Sanitize the falloff parameters. Zero or out-of-range values
	// fall back to safe defaults so this function is well-defined even
	// when called from code that leaves the fields unset (tests that
	// build runOptions directly, etc.).
	if intensitySoftness <= 0 {
		intensitySoftness = 1.0
	}
	if intensitySoftness > 2.0 {
		intensitySoftness = 2.0
	}
	if intensityMinRatio < 0 {
		intensityMinRatio = 0
	}
	if intensityMinRatio > 1 {
		intensityMinRatio = 1
	}
	if lengthSoftness <= 0 {
		lengthSoftness = 1.0
	}
	if lengthSoftness > 4.0 {
		lengthSoftness = 4.0
	}
	if lengthMinRatio < 0 {
		lengthMinRatio = 0
	}
	if lengthMinRatio > 1 {
		lengthMinRatio = 1
	}
	lengthCap := float32(f.Width) / 2
	if float32(f.Height)/2 < lengthCap {
		lengthCap = float32(f.Height) / 2
	}
	intensityCap := float32(baseIntensity)
	if intensityCap > 2 {
		intensityCap = 2
	}

	// Gaussian cross-section perpendicular to each arm, sigma = 1.5 px.
	// Sweep 3 px perpendicular so the arm has visible width without
	// becoming a fat band.
	const perpSigma = 1.5
	perpVar := 2.0 * perpSigma * perpSigma
	const perpBand = 3

	result := &frame{Width: f.Width, Height: f.Height, Pix: make([]float32, len(f.Pix))}
	copy(result.Pix, f.Pix)

	for _, s := range stars {
		relFlux := s.Flux / maxFlux
		if relFlux <= 0 {
			continue
		}
		// Two independent power-law falloffs:
		//
		// intensityWeight = relFlux ^ intensitySoftness
		//   controls brightness of the spike. The default
		//   intensitySoftness=0.5 compresses the range so faint
		//   spikes are still visible, and intensityMinRatio=0.2
		//   acts as a floor. This means a star at 1% of peak flux
		//   still gets a 20% spike, not a near-zero spike.
		//
		// lengthWeight = relFlux ^ lengthSoftness
		//   controls arm length. The default lengthSoftness=1.0 is
		//   strictly proportional (a star half as bright as the peak
		//   gets a half-length spike). This is what makes the visual
		//   hierarchy of the frame work: bright stars dominate with
		//   long spikes, faint stars have short spikes. Use
		//   lengthSoftness=1.5 or 2.0 for even more differentiation.
		intensityWeight := math.Pow(relFlux, intensitySoftness)
		if intensityWeight < intensityMinRatio {
			intensityWeight = intensityMinRatio
		}
		lengthWeight := math.Pow(relFlux, lengthSoftness)
		if lengthWeight < lengthMinRatio {
			lengthWeight = lengthMinRatio
		}
		starLength := float32(baseLength * lengthWeight)
		if starLength > lengthCap {
			starLength = lengthCap
		}
		if starLength < 1 {
			continue
		}
		starIntensity := intensityCap * float32(intensityWeight)
		cx, cy := float32(s.X), float32(s.Y)
		for arm := 0; arm < numArms; arm++ {
			angle := baseAngle + float64(arm)*2*math.Pi/float64(numArms)
			dx := float32(math.Cos(angle))
			dy := float32(math.Sin(angle))
			for t := float32(1); t < starLength; t += 1.0 {
				armFade := 1.0 - t/starLength
				if armFade <= 0 {
					break
				}
				px := cx + dx*t
				py := cy + dy*t
				ix := int(px + 0.5)
				iy := int(py + 0.5)
				if ix < 0 || iy < 0 || ix >= f.Width || iy >= f.Height {
					break
				}
				for pOff := -perpBand; pOff <= perpBand; pOff++ {
					px2 := px - dy*float32(pOff)
					py2 := py + dx*float32(pOff)
					ix2 := int(px2 + 0.5)
					iy2 := int(py2 + 0.5)
					if ix2 < 0 || iy2 < 0 || ix2 >= f.Width || iy2 >= f.Height {
						continue
					}
					perpFade := float32(math.Exp(-float64(pOff*pOff) / perpVar))
					add := starIntensity * armFade * perpFade
					if add < 0.005 {
						continue
					}
					i := (iy2*f.Width + ix2) * 3
					for c := 0; c < 3; c++ {
						v := result.Pix[i+c] + add
						if v > 1 {
							v = 1
						}
						result.Pix[i+c] = v
					}
				}
			}
		}
	}
	return result
}

func floatToByte(v float32) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}

// writeLinearPreviewJPEG maps [0,1] values directly to [0,255] with no
// gamma. Use for post-processing stages whose outputs are already stretched.
func writeLinearPreviewJPEG(w io.Writer, width, height int, pix []float32) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		rowOff := y * img.Stride
		for x := 0; x < width; x++ {
			i := (y*width + x) * 3
			o := rowOff + x*4
			img.Pix[o+0] = floatToByte(pix[i+0])
			img.Pix[o+1] = floatToByte(pix[i+1])
			img.Pix[o+2] = floatToByte(pix[i+2])
			img.Pix[o+3] = 255
		}
	}
	return jpeg.Encode(w, img, &jpeg.Options{Quality: 90})
}

// writeAutoNormalizedLinearPreviewJPEG divides by the global max then maps
// [0,1] to [0,255] linearly. Use when values may be slightly outside [0,1]
// (step 1) but gamma lifting would wash out the background.
func writeAutoNormalizedLinearPreviewJPEG(w io.Writer, width, height int, pix []float32) error {
	maxVal := float32(0)
	for _, v := range pix {
		if v > maxVal {
			maxVal = v
		}
	}
	if math.IsNaN(float64(maxVal)) || math.IsInf(float64(maxVal), 0) || maxVal <= 0 {
		maxVal = 1.0
	}
	inv := float32(1.0) / maxVal
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		rowOff := y * img.Stride
		for x := 0; x < width; x++ {
			i := (y*width + x) * 3
			o := rowOff + x*4
			img.Pix[o+0] = floatToByte(pix[i+0] * inv)
			img.Pix[o+1] = floatToByte(pix[i+1] * inv)
			img.Pix[o+2] = floatToByte(pix[i+2] * inv)
			img.Pix[o+3] = 255
		}
	}
	return jpeg.Encode(w, img, &jpeg.Options{Quality: 90})
}

func writePostPreview(dir, name string, f *frame) error {
	return writeFileAtomic(filepath.Join(dir, name), func(w io.Writer) error {
		return writeAutoNormalizedLinearPreviewJPEG(w, f.Width, f.Height, f.Pix)
	})
}

func runPostProcessing(outputDir string, opts runOptions) ([]string, error) {
	workPath := filepath.Join(outputDir, "_work", "stacked_mean.f32raw")
	w, h, pix, err := readRawFloat32RGB(workPath)
	if err != nil {
		return nil, fmt.Errorf("post-process: read work file: %w", err)
	}
	src := &frame{Width: w, Height: h, Pix: pix}

	var outputs []string
	cur := src
	var step1Frame, step4Frame, step5Frame, step6Frame, step7Frame *frame

	// Step 1: background removal (ALWAYS)
	logf("  post: background removal (degree %d)", opts.BGDegree)
	cur = removeBackground(cur, opts.BGDegree, opts.MaskBG)
	step1Frame = cur
	p := filepath.Join(outputDir, "post_step1_background_removed.jpg")
	if err := writePostPreview(outputDir, filepath.Base(p), cur); err != nil {
		logf("  WARNING: %v", err)
	}
	outputs = append(outputs, p)

	if opts.PostCA {
		logf("  post: chromatic aberration reduction")
		cur = suppressChromaticAberration(cur)
		p = filepath.Join(outputDir, "post_step_ca.jpg")
		if err := writePostPreview(outputDir, filepath.Base(p), cur); err != nil {
			logf("  WARNING: %v", err)
		}
		outputs = append(outputs, p)
	}

	if opts.PostColor {
		logf("  post: color calibration")
		cur = calibrateColor(cur)
		p = filepath.Join(outputDir, "post_step_color_calibrated.jpg")
		if err := writePostPreview(outputDir, filepath.Base(p), cur); err != nil {
			logf("  WARNING: %v", err)
		}
		outputs = append(outputs, p)
	}

	if opts.PostFlatten {
		logf("  post: vignette flattening")
		cur = flattenBackground(cur, 2, opts.MaskBG)
		p = filepath.Join(outputDir, "post_step_flattened.jpg")
		if err := writePostPreview(outputDir, filepath.Base(p), cur); err != nil {
			logf("  WARNING: %v", err)
		}
		outputs = append(outputs, p)
	}

	if opts.PostSharpen {
		logf("  post: unsharp mask (amount=%g radius=%g)", opts.SharpenAmount, opts.SharpenRadius)
		cur = unsharpMask(cur, opts.SharpenAmount, opts.SharpenRadius)
		p = filepath.Join(outputDir, "post_step_sharpened.jpg")
		if err := writePostPreview(outputDir, filepath.Base(p), cur); err != nil {
			logf("  WARNING: %v", err)
		}
		outputs = append(outputs, p)
	}

	if opts.PostHDR {
		logf("  post: HDR local contrast (amount=%g)", opts.HDRAmount)
		cur = applyHDR(cur, opts.HDRAmount)
		p = filepath.Join(outputDir, "post_step_hdr.jpg")
		if err := writePostPreview(outputDir, filepath.Base(p), cur); err != nil {
			logf("  WARNING: %v", err)
		}
		outputs = append(outputs, p)
	}

	// Star removal (ALWAYS): replace detected stars with local background
	logf("  post: star removal (sigma=%g, radius=%d)", opts.StarSigma, opts.StarRadius)
	cur = removeStars(cur, opts.StarSigma, opts.StarRadius)
	p = filepath.Join(outputDir, "post_step2_star_removed.jpg")
	if err := writePostPreview(outputDir, filepath.Base(p), cur); err != nil {
		logf("  WARNING: %v", err)
	}
	outputs = append(outputs, p)

	// Black point polish (ALWAYS, unless --bg-percentile 0): subtract the
	// configured percentile of luminance so the darkest parts of the image
	// are truly black. Combined with the star removal above, this gives the
	// "near-dark background" that makes faint stars disappear into the sky.
	if opts.BgPercentile > 0 {
		logf("  post: black point shift (P%g, intermediate, not written)", opts.BgPercentile)
		cur = blackPointShift(cur, opts.BgPercentile)
		// post_step3_dark_background.jpg is intentionally NOT written:
		// in typical runs the black-point shift is tiny (P10 near
		// -0.003), so the resulting JPEG is visually identical to
		// post_step2. The transformation still runs because step 4
		// (auto-stretch) consumes its output.
	}

	// Auto-stretch (FINAL): PixInsight-style STF/MTF.  Lifts the
	// background to a target level and reveals faint detail without
	// clipping bright stars.  Disable with --no-auto-stretch.
	if !opts.NoAutoStretch {
		logf("  post: auto-stretch (STF: shadows=%.1f target=%.2f linked=%v)",
			opts.STFShadowsClip, opts.STFTargetBG, opts.STFLinked)
		stretched := autoStretch(cur, opts.STFShadowsClip, opts.STFTargetBG, opts.STFLinked)
		step4Frame = stretched
		p = filepath.Join(outputDir, "post_step4_auto_stretched.jpg")
		if err := writePostPreview(outputDir, filepath.Base(p), stretched); err != nil {
			logf("  WARNING: %v", err)
		}
		outputs = append(outputs, p)
	}

	// Step 5: denoise the auto-stretched frame. The guided filter
	// smooths flat regions while preserving edges and stars. Skippable
	// with --no-denoise, tunable with --denoise-strength / --denoise-radius.
	step5Frame = step4Frame
	if step4Frame != nil && !opts.NoDenoise && opts.DenoiseStrength > 0 {
		logf("  post: denoise (guided filter, radius=%d, strength=%g)",
			opts.DenoiseRadius, opts.DenoiseStrength)
		denoised := denoiseFrame(step4Frame, opts.DenoiseRadius, 0.01, opts.DenoiseStrength, opts.DenoiseChroma)
		step5Frame = denoised
		p = filepath.Join(outputDir, "post_step5_denoised.jpg")
		if err := writePostPreview(outputDir, filepath.Base(p), denoised); err != nil {
			logf("  WARNING: %v", err)
		}
		outputs = append(outputs, p)
	} else {
		logf("  post: denoise skipped")
	}

	// Step 6 (NEW): curves. Darken the background by crushing the
	// deepest shadows and pushing midtones down slightly, while
	// preserving highlights. This is the equivalent of a GIMP Curves
	// adjustment with an S-shaped curve; the effect is a deeper black
	// floor and richer shadow contrast. Both parameters are applied
	// identically to each channel so hue is preserved.
	step6Frame = step5Frame
	if step5Frame != nil && !opts.NoCurves &&
		(opts.CurvesBlack > 0 || math.Abs(opts.CurvesGamma-1.0) > 1e-6) {
		logf("  post: curves (black=%g gamma=%g)", opts.CurvesBlack, opts.CurvesGamma)
		curved := applyCurves(step5Frame, opts.CurvesBlack, opts.CurvesGamma)
		step6Frame = curved
		p = filepath.Join(outputDir, "post_step6_curves.jpg")
		if err := writePostPreview(outputDir, filepath.Base(p), curved); err != nil {
			logf("  WARNING: %v", err)
		}
		outputs = append(outputs, p)
	} else {
		logf("  post: curves skipped")
	}

	// Step 7 (HDR): add the display base (curves output from step 6,
	// or step 4 if curves is disabled) to the background-removed frame
	// (step 1). The display base gives the overall tone; the linear
	// detail recovers bright-core and high-contrast regions that the
	// auto-stretch compressed.
	if step1Frame != nil && step6Frame != nil && !opts.NoHDRBlend && opts.HDRBlendFactor > 0 {
		logf("  post: HDR blend (base + %g*step1)", opts.HDRBlendFactor)
		blended := blendHDR(step6Frame, step1Frame, opts.HDRBlendFactor)
		step7Frame = blended
		p = filepath.Join(outputDir, "post_step7_hdr.jpg")
		if err := writePostPreview(outputDir, filepath.Base(p), blended); err != nil {
			logf("  WARNING: %v", err)
		}
		outputs = append(outputs, p)
	} else if opts.NoAutoStretch {
		logf("  post: HDR blend skipped (auto-stretch disabled)")
	} else if opts.NoHDRBlend {
		logf("  post: HDR blend skipped (--no-hdr-blend)")
	}

	// Step 8: weighted mix of step 1 (background removed) and step 7 (HDR).
	// Produces four variants spanning from "mostly step 1" to "mostly
	// step 7", so the user can pick whichever balance of linear detail
	// and HDR-processed tone looks best on their image.
	if step1Frame != nil && step7Frame != nil && !opts.NoHDRMix {
		weights := []float64{0.90, 0.80, 0.70, 0.60}
		logf("  post: HDR mix (%d weighted variants: step8_1..step8_%d)", len(weights), len(weights))

		// If spikes are requested, detect stars ONCE (using the HDR base
		// step7Frame as the reference) and generate a single random base
		// angle. Reusing the same star list and base angle across all
		// four variants means post_step8_1..4 show spikes on the same
		// stars, at the same orientation, with the same brightness
		// ordering. Without this the four outputs would spike slightly
		// different stars because their relative fluxes differ.
		var spikedStars []star
		var spikesBaseAngle float64
		if opts.AddSpikes {
			spikedStars = selectSpikeStars(step7Frame, opts.SpikesPercent, opts.SpikesMinCount)
			if len(spikedStars) > 0 {
				rng := rand.New(rand.NewSource(time.Now().UnixNano()))
				spikesBaseAngle = rng.Float64() * 2 * math.Pi
				logf("  spikes: %d star(s) selected (base angle = %.1f°, arms=%d)",
					len(spikedStars), spikesBaseAngle*180/math.Pi, opts.SpikesArms)
			} else {
				logf("  spikes: no stars detected, skipping")
			}
		}

		for i, w1 := range weights {
			mixed := blendWeighted(step1Frame, step7Frame, w1)
			if opts.AddSpikes && len(spikedStars) > 0 {
				mixed = overlaySpikes(mixed, spikedStars, opts.SpikesLength, opts.SpikesIntensity, opts.SpikesArms, spikesBaseAngle, opts.SpikesSoftness, opts.SpikesMinRatio, opts.SpikesLengthSoftness, opts.SpikesLengthMinRatio)
			}
			p = filepath.Join(outputDir, fmt.Sprintf("post_step8_%d.jpg", i+1))
			if err := writePostPreview(outputDir, filepath.Base(p), mixed); err != nil {
				logf("  WARNING: %v", err)
			}
			outputs = append(outputs, p)
			logf("  post: step8_%d (%.2f*step1 + %.2f*step7)", i+1, w1, 1-w1)
		}
	} else if opts.NoHDRMix {
		logf("  post: HDR mix skipped (--no-hdr-mix)")
	} else if step7Frame == nil {
		logf("  post: HDR mix skipped (step 7 not produced)")
	}

	if opts.PostStretch {
		logf("  post: stretch gentle")
		s := stretchContrast(cur, 1)
		p = filepath.Join(outputDir, "post_step_stretch_gentle.jpg")
		if err := writePostPreview(outputDir, filepath.Base(p), s); err != nil {
			logf("  WARNING: %v", err)
		}
		outputs = append(outputs, p)
		logf("  post: stretch moderate")
		s = stretchContrast(cur, 2)
		p = filepath.Join(outputDir, "post_step_stretch_moderate.jpg")
		if err := writePostPreview(outputDir, filepath.Base(p), s); err != nil {
			logf("  WARNING: %v", err)
		}
		outputs = append(outputs, p)
		logf("  post: stretch full")
		s = stretchContrast(cur, 3)
		p = filepath.Join(outputDir, "post_step_stretch_full.jpg")
		if err := writePostPreview(outputDir, filepath.Base(p), s); err != nil {
			logf("  WARNING: %v", err)
		}
		outputs = append(outputs, p)
	}

	return outputs, nil
}

// =========================================================================
// POST-PROCESSING SELF-TESTS
// =========================================================================

func parseFITSHeader(path string) (width, height, naxis3, bitpix int, bscale, bzero float64, offset int64, baypat, roworder string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	buf := make([]byte, 80)
	bscale = 1.0
	bzero = 0.0
	for {
		if _, err = io.ReadFull(f, buf); err != nil {
			break
		}
		card := string(buf)
		if strings.HasPrefix(card, "END") {
			cur, _ := f.Seek(0, io.SeekCurrent)
			pad := (2880 - (cur % 2880)) % 2880
			offset = cur + pad
			break
		}
		key := strings.TrimSpace(card[0:8])
		val := strings.TrimSpace(card[10:30])
		switch key {
		case "NAXIS1":
			width, _ = strconv.Atoi(val)
		case "NAXIS2":
			height, _ = strconv.Atoi(val)
		case "NAXIS3":
			naxis3, _ = strconv.Atoi(val)
		case "BITPIX":
			bitpix, _ = strconv.Atoi(val)
		case "BSCALE":
			bscale, _ = strconv.ParseFloat(val, 64)
		case "BZERO":
			bzero, _ = strconv.ParseFloat(val, 64)
		case "BAYERPAT":
			baypat = strings.TrimSpace(strings.Trim(val, "'"))
		case "ROWORDER":
			roworder = strings.TrimSpace(strings.Trim(val, "'"))
		}
	}
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0, 1, 0, 0, "", "", fmt.Errorf("invalid FITS header")
	}
	return
}

func decodeFITSNative(path string, targetW, targetH int) (*frame, error) {
	w, h, naxis3, bitpix, bscale, bzero, offset, baypat, roworder, err := parseFITSHeader(path)
	if err != nil {
		return nil, err
	}

	var bytesPerPixel int
	switch bitpix {
	case 8:
		bytesPerPixel = 1
	case 16:
		bytesPerPixel = 2
	case 32:
		bytesPerPixel = 4
	case -32:
		bytesPerPixel = 4
	case -64:
		bytesPerPixel = 8
	default:
		return nil, fmt.Errorf("unsupported BITPIX %d", bitpix)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}

	nPix := w * h
	numPlanes := 1
	if naxis3 > 1 {
		numPlanes = naxis3
	}

	data := make([]byte, nPix*numPlanes*bytesPerPixel)
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, err
	}

	frm := &frame{Width: w, Height: h, Pix: make([]float32, nPix*3)}

	vals := make([]float32, nPix*numPlanes)
	for i := 0; i < nPix*numPlanes; i++ {
		var v float64
		switch bitpix {
		case 8:
			v = float64(data[i])
		case 16:
			v = float64(int16(binary.BigEndian.Uint16(data[i*2:])))
		case 32:
			v = float64(int32(binary.BigEndian.Uint32(data[i*4:])))
		case -32:
			v = float64(math.Float32frombits(binary.BigEndian.Uint32(data[i*4:])))
		case -64:
			v = math.Float64frombits(binary.BigEndian.Uint64(data[i*8:]))
		}
		vals[i] = float32(v*bscale + bzero)
	}

	for p := 0; p < numPlanes; p++ {
		minVal, maxVal := float32(1e30), float32(-1e30)
		for i := 0; i < nPix; i++ {
			v := vals[p*nPix+i]
			if v < minVal {
				minVal = v
			}
			if v > maxVal {
				maxVal = v
			}
		}
		rangeVal := maxVal - minVal
		if rangeVal <= 0 {
			rangeVal = 1.0
		}
		for i := 0; i < nPix; i++ {
			v := (vals[p*nPix+i] - minVal) / rangeVal
			if v < 0 {
				v = 0
			} else if v > 1 {
				v = 1
			}
			if numPlanes == 1 {
				frm.Pix[i*3] = v
				frm.Pix[i*3+1] = v
				frm.Pix[i*3+2] = v
			} else {
				frm.Pix[i*3+p] = v
			}
		}
	}

	if numPlanes == 1 && baypat != "" {
		return debayerFITS(vals, w, h, baypat, roworder, targetW, targetH), nil
	}

	if targetW > 0 && targetH > 0 && (w != targetW || h != targetH) {
		frm = resizeFrame(frm, targetW, targetH)
	}
	return frm, nil
}

func debayerFITS(vals []float32, w, h int, baypat, roworder string, targetW, targetH int) *frame {
	if roworder == "BOTTOM-UP" {
		for y := 0; y < h/2; y++ {
			for x := 0; x < w; x++ {
				idx1 := y*w + x
				idx2 := (h-1-y)*w + x
				vals[idx1], vals[idx2] = vals[idx2], vals[idx1]
			}
		}
		switch baypat {
		case "RGGB":
			baypat = "GBRG"
		case "BGGR":
			baypat = "GRBG"
		case "GRBG":
			baypat = "BGGR"
		case "GBRG":
			baypat = "RGGB"
		}
	}

	minVal, maxVal := float32(1e30), float32(-1e30)
	for _, v := range vals {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	rangeVal := maxVal - minVal
	if rangeVal <= 0 {
		rangeVal = 1.0
	}
	norm := make([]float32, len(vals))
	for i, v := range vals {
		norm[i] = (v - minVal) / rangeVal
	}

	var cfa [2][2]int
	switch baypat {
	case "RGGB":
		cfa = [2][2]int{{0, 1}, {1, 2}}
	case "BGGR":
		cfa = [2][2]int{{2, 1}, {1, 0}}
	case "GRBG":
		cfa = [2][2]int{{1, 0}, {2, 1}}
	case "GBRG":
		cfa = [2][2]int{{1, 2}, {0, 1}}
	default:
		frm := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
		for i := 0; i < w*h; i++ {
			frm.Pix[i*3] = norm[i]
			frm.Pix[i*3+1] = norm[i]
			frm.Pix[i*3+2] = norm[i]
		}
		if targetW > 0 && targetH > 0 && (w != targetW || h != targetH) {
			return resizeFrame(frm, targetW, targetH)
		}
		return frm
	}

	frm := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	getV := func(x, y int) float32 {
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		if x >= w {
			x = w - 1
		}
		if y >= h {
			y = h - 1
		}
		return norm[y*w+x]
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := cfa[y%2][x%2]
			var r, g, b float32
			if c == 0 { // R
				r = getV(x, y)
				g = (getV(x-1, y) + getV(x+1, y) + getV(x, y-1) + getV(x, y+1)) * 0.25
				b = (getV(x-1, y-1) + getV(x+1, y-1) + getV(x-1, y+1) + getV(x+1, y+1)) * 0.25
			} else if c == 2 { // B
				b = getV(x, y)
				g = (getV(x-1, y) + getV(x+1, y) + getV(x, y-1) + getV(x, y+1)) * 0.25
				r = (getV(x-1, y-1) + getV(x+1, y-1) + getV(x-1, y+1) + getV(x+1, y+1)) * 0.25
			} else { // G
				g = getV(x, y)
				if cfa[y%2][(x+1)%2] == 0 { // R is horizontally adjacent
					r = (getV(x-1, y) + getV(x+1, y)) * 0.5
					b = (getV(x, y-1) + getV(x, y+1)) * 0.5
				} else { // B is horizontally adjacent
					b = (getV(x-1, y) + getV(x+1, y)) * 0.5
					r = (getV(x, y-1) + getV(x, y+1)) * 0.5
				}
			}
			frm.Pix[(y*w+x)*3] = r
			frm.Pix[(y*w+x)*3+1] = g
			frm.Pix[(y*w+x)*3+2] = b
		}
	}

	if targetW > 0 && targetH > 0 && (w != targetW || h != targetH) {
		return resizeFrame(frm, targetW, targetH)
	}
	return frm
}

// testFITSFileDebayer is the --test-debayer CLI backend: decode one FITS
// file with debayering, write the result as <base>.jpg next to the source,
// and write <base>_raw.jpg showing the raw CFA pattern as false-color
// (R pixels red, G pixels green, B pixels blue), in the orientation stored
// in the file (before any ROWORDER flip). Header fields are printed to
// stderr so the user can confirm the header was parsed correctly.
//
// Diagnostics that this function is designed to expose:
//   - raw _raw.jpg shows a clean R/G/B checkerboard but .jpg is grayscale
//     -> debayerFITS did not run (BAYERPAT missing or wrong pattern)
//   - .jpg shows the wrong hue (e.g. red nebula looks blue)
//     -> BAYERPAT or ROWORDER does not match the sensor
//   - .jpg shows the correct hue and clean color
//     -> debayering is correct; any remaining issue is in the pipeline
func testFITSFileDebayer(path string) error {
	// 1. Header
	w, h, naxis3, bitpix, bscale, bzero, offset, baypat, roworder, err := parseFITSHeader(path)
	if err != nil {
		return fmt.Errorf("read header: %w", err)
	}
	logf("FITS: %s", path)
	logf("  dimensions: %d x %d  (NAXIS3=%d)", w, h, naxis3)
	logf("  BITPIX=%d  BSCALE=%g  BZERO=%g  data offset=%d", bitpix, bscale, bzero, offset)
	logf("  BAYERPAT=%q  ROWORDER=%q", baypat, roworder)
	if baypat == "" {
		logf("  WARNING: BAYERPAT is missing from the header; the pipeline will emit grayscale, not debayered output")
	}

	// 2. Raw data
	var bytesPerPixel int
	switch bitpix {
	case 8:
		bytesPerPixel = 1
	case 16:
		bytesPerPixel = 2
	case 32:
		bytesPerPixel = 4
	case -32:
		bytesPerPixel = 4
	case -64:
		bytesPerPixel = 8
	default:
		return fmt.Errorf("unsupported BITPIX %d", bitpix)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	nPix := w * h
	numPlanes := 1
	if naxis3 > 1 {
		numPlanes = naxis3
	}
	if numPlanes != 1 {
		return fmt.Errorf("test-debayer expects mono data, got NAXIS3=%d", naxis3)
	}
	data := make([]byte, nPix*bytesPerPixel)
	if _, err := io.ReadFull(f, data); err != nil {
		return fmt.Errorf("read data: %w", err)
	}
	vals := make([]float32, nPix)
	for i := 0; i < nPix; i++ {
		var v float64
		switch bitpix {
		case 8:
			v = float64(data[i])
		case 16:
			v = float64(int16(binary.BigEndian.Uint16(data[i*2:])))
		case 32:
			v = float64(int32(binary.BigEndian.Uint32(data[i*4:])))
		case -32:
			v = float64(math.Float32frombits(binary.BigEndian.Uint32(data[i*4:])))
		case -64:
			v = math.Float64frombits(binary.BigEndian.Uint64(data[i*8:]))
		}
		vals[i] = float32(v*bscale + bzero)
	}

	// 3. Min/max for normalization (shared by raw preview and debayer)
	minVal, maxVal := float32(1e30), float32(-1e30)
	for _, v := range vals {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	rng := maxVal - minVal
	if rng <= 0 {
		rng = 1.0
	}
	logf("  value range: min=%g  max=%g  range=%g", minVal, maxVal, rng)

	// 4. Raw CFA pattern as false-color (R->red, G->green, B->blue) using
	// the header's pattern, as stored (before any ROWORDER flip).
	rawPix := make([]float32, nPix*3)
	var cfa [2][2]int
	haveCFA := true
	switch baypat {
	case "RGGB":
		cfa = [2][2]int{{0, 1}, {1, 2}}
	case "BGGR":
		cfa = [2][2]int{{2, 1}, {1, 0}}
	case "GRBG":
		cfa = [2][2]int{{1, 0}, {2, 1}}
	case "GBRG":
		cfa = [2][2]int{{1, 2}, {0, 1}}
	default:
		haveCFA = false
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			n := (vals[i] - minVal) / rng
			if haveCFA {
				switch cfa[y%2][x%2] {
				case 0:
					rawPix[i*3] = n
				case 1:
					rawPix[i*3+1] = n
				case 2:
					rawPix[i*3+2] = n
				}
			} else {
				rawPix[i*3] = n
				rawPix[i*3+1] = n
				rawPix[i*3+2] = n
			}
		}
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	rawPath := base + "_raw.jpg"
	if err := writeFileAtomic(rawPath, func(wr io.Writer) error {
		return writeLinearPreviewJPEG(wr, w, h, rawPix)
	}); err != nil {
		return fmt.Errorf("write raw preview: %w", err)
	}
	if haveCFA {
		logf("  wrote raw CFA false-color (pattern %s as stored): %s", baypat, rawPath)
	} else {
		logf("  wrote raw CFA grayscale (no BAYERPAT): %s", rawPath)
	}

	// 5. Debayer. debayerFITS mutates vals for ROWORDER, so pass a copy.
	var frm *frame
	if baypat != "" {
		valsCopy := make([]float32, len(vals))
		copy(valsCopy, vals)
		frm = debayerFITS(valsCopy, w, h, baypat, roworder, 0, 0)
	} else {
		frm = &frame{Width: w, Height: h, Pix: rawPix}
	}

	// 6. Sanity: does the output have real channel separation?
	var maxRG, maxRB float32
	for i := 0; i < len(frm.Pix); i += 3 {
		rg := frm.Pix[i] - frm.Pix[i+1]
		if rg < 0 {
			rg = -rg
		}
		rb := frm.Pix[i] - frm.Pix[i+2]
		if rb < 0 {
			rb = -rb
		}
		if rg > maxRG {
			maxRG = rg
		}
		if rb > maxRB {
			maxRB = rb
		}
	}
	if maxRG < 0.01 && maxRB < 0.01 {
		logf("  WARNING: output channels are near-identical (max |R-G|=%.4f |R-B|=%.4f); debayering may not have run", maxRG, maxRB)
	} else {
		logf("  channel separation: max |R-G|=%.4f  |R-B|=%.4f  (debayering active)", maxRG, maxRB)
	}

	// 7. Debayered JPG
	outPath := base + ".jpg"
	if err := writeFileAtomic(outPath, func(wr io.Writer) error {
		return writeAutoNormalizedLinearPreviewJPEG(wr, frm.Width, frm.Height, frm.Pix)
	}); err != nil {
		return fmt.Errorf("write debayered preview: %w", err)
	}
	logf("  wrote debayered RGB: %s", outPath)
	return nil
}

// testDebayerCLI: the --test-debayer flag must produce both output files
// (raw CFA and debayered RGB) for a valid RGGB FITS file.
func testDebayerCLI() error {
	dir, err := os.MkdirTemp("", "skystacker-test-debayer-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	const w, h = 32, 32
	var buf bytes.Buffer
	header := []string{
		"SIMPLE  =                    T",
		"BITPIX  =                   16",
		"NAXIS   =                    2",
		"NAXIS1  =                   32",
		"NAXIS2  =                   32",
		"BSCALE  =                  1.0",
		"BZERO   =                  0.0",
		"BAYERPAT= 'RGGB    '",
		"END",
	}
	for _, line := range header {
		buf.WriteString(fmt.Sprintf("%-80s", line))
	}
	for buf.Len()%2880 != 0 {
		buf.WriteByte(' ')
	}
	payload := make([]byte, w*h*2)
	cfa := [2][2]int{{0, 1}, {1, 2}}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var v uint16
			switch cfa[y%2][x%2] {
			case 0:
				v = 1000
			case 1:
				v = 500
			case 2:
				v = 100
			}
			binary.BigEndian.PutUint16(payload[(y*w+x)*2:], v)
		}
	}
	buf.Write(payload)
	path := filepath.Join(dir, "test.fits")
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return err
	}
	if err := testFITSFileDebayer(path); err != nil {
		return fmt.Errorf("testFITSFileDebayer: %w", err)
	}
	for _, name := range []string{"test.jpg", "test_raw.jpg"} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("missing output %s: %w", name, err)
		}
		if fi.Size() == 0 {
			return fmt.Errorf("empty output %s", name)
		}
	}
	return nil
}

func testFITSNative() error {
	dir, err := os.MkdirTemp("", "skystacker-fits-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "test.fits")
	var buf bytes.Buffer
	header := []string{
		"SIMPLE  =                    T",
		"BITPIX  =                   16",
		"NAXIS   =                    2",
		"NAXIS1  =                   10",
		"NAXIS2  =                   10",
		"BSCALE  =                  1.0",
		"BZERO   =                  0.0",
		"END",
	}
	for _, line := range header {
		buf.WriteString(fmt.Sprintf("%-80s", line))
	}
	for buf.Len()%2880 != 0 {
		buf.WriteByte(' ')
	}
	payload := make([]byte, 200)
	for i := 0; i < 100; i++ {
		binary.BigEndian.PutUint16(payload[i*2:], uint16(i))
	}
	buf.Write(payload)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return err
	}
	f, err := decodeFITSNative(path, 0, 0)
	if err != nil {
		return err
	}
	if f.Width != 10 || f.Height != 10 {
		return fmt.Errorf("FITS: got %dx%d, want 10x10", f.Width, f.Height)
	}
	if f.Pix[0] != 0 {
		return fmt.Errorf("FITS pixel 0 = %v, want 0", f.Pix[0])
	}
	if f.Pix[99*3] != 1.0 {
		return fmt.Errorf("FITS pixel 99 = %v, want 1.0", f.Pix[99*3])
	}
	return nil
}

func testFITSDebayer() error {
	dir, err := os.MkdirTemp("", "skystacker-fits-debayer-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "test_bayer.fits")
	var buf bytes.Buffer
	header := []string{
		"SIMPLE  =                    T",
		"BITPIX  =                   16",
		"NAXIS   =                    2",
		"NAXIS1  =                   10",
		"NAXIS2  =                   10",
		"BSCALE  =                  1.0",
		"BZERO   =                  0.0",
		"BAYERPAT= 'RGGB    '",
		"ROWORDER= 'BOTTOM-UP'",
		"END",
	}
	for _, line := range header {
		buf.WriteString(fmt.Sprintf("%-80s", line))
	}
	for buf.Len()%2880 != 0 {
		buf.WriteByte(' ')
	}
	payload := make([]byte, 200)
	for i := 0; i < 100; i++ {
		binary.BigEndian.PutUint16(payload[i*2:], uint16(i))
	}
	buf.Write(payload)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return err
	}
	f, err := decodeFITSNative(path, 0, 0)
	if err != nil {
		return err
	}
	if f.Width != 10 || f.Height != 10 {
		return fmt.Errorf("FITS debayer: got %dx%d, want 10x10", f.Width, f.Height)
	}
	return nil
}

func testPolyBasis() error {
	b := polyBasis(2, 3, 2)
	want := []float64{1, 2, 3, 4, 6, 9}
	if len(b) != len(want) {
		return fmt.Errorf("len=%d want %d", len(b), len(want))
	}
	for i := range want {
		if math.Abs(b[i]-want[i]) > 1e-9 {
			return fmt.Errorf("b[%d]=%v want %v", i, b[i], want[i])
		}
	}
	b3 := polyBasis(2, 3, 3)
	if len(b3) != 10 {
		return fmt.Errorf("degree 3 len=%d want 10", len(b3))
	}
	if nCoeff(1) != 3 || nCoeff(2) != 6 || nCoeff(3) != 10 {
		return fmt.Errorf("nCoeff wrong")
	}
	return nil
}

func testSolveLinear() error {
	// 2×2 system: 2x + y = 5; x + 3y = 10  → x=1, y=3
	A := [][]float64{{2, 1}, {1, 3}}
	b := []float64{5, 10}
	x := solveLinear(A, b, 2)
	if math.Abs(x[0]-1) > 1e-9 || math.Abs(x[1]-3) > 1e-9 {
		return fmt.Errorf("got %v, want [1 3]", x)
	}
	return nil
}

func testBackgroundRemoval() error {
	const w, h = 128, 128
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	// Linear gradient background: 0.10 → 0.60
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			bg := float32(0.01 + 0.04*float64(x)/float64(w) + 0.02*float64(y)/float64(h))
			i := (y*w + x) * 3
			f.Pix[i] = bg
			f.Pix[i+1] = bg
			f.Pix[i+2] = bg
		}
	}
	// A few synthetic stars
	stars := []point2D{{20, 20}, {100, 30}, {60, 90}, {30, 100}, {110, 110}}
	for _, s := range stars {
		for dy := -2; dy <= 2; dy++ {
			for dx := -2; dx <= 2; dx++ {
				x, y := int(s.X)+dx, int(s.Y)+dy
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				d2 := float64(dx*dx + dy*dy)
				v := float32(0.5 * math.Exp(-d2))
				i := (y*w + x) * 3
				f.Pix[i] += v
				f.Pix[i+1] += v
				f.Pix[i+2] += v
			}
		}
	}
	result := removeBackground(f, 2, false)
	// Residuals should be small away from stars
	var sumRes float64
	var nRes int
	for y := 8; y < h-8; y += 8 {
		for x := 8; x < w-8; x += 8 {
			nearStar := false
			for _, s := range stars {
				if math.Hypot(float64(x)-s.X, float64(y)-s.Y) < 5 {
					nearStar = true
					break
				}
			}
			if nearStar {
				continue
			}
			i := (y*w + x) * 3
			sumRes += math.Abs(float64(result.Pix[i]))
			nRes++
		}
	}
	if nRes == 0 {
		return errors.New("no residual samples")
	}
	avg := sumRes / float64(nRes)
	if avg > 0.02 {
		return fmt.Errorf("avg residual %.4f too high (want < 0.02)", avg)
	}
	return nil
}

func testContrastStretch() error {
	const w, h = 100, 100
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w*h; i++ {
		var v float32
		switch {
		case i%100 == 0:
			v = 0.5
		case i%10 == 0:
			v = 0.2
		default:
			v = 0.1
		}
		f.Pix[i*3] = v
		f.Pix[i*3+1] = v
		f.Pix[i*3+2] = v
	}
	result := stretchContrast(f, 3)
	var bgSum, midSum, brSum float32
	var nBg, nMid, nBr int
	for i := 0; i < w*h; i++ {
		switch {
		case i%100 == 0:
			brSum += result.Pix[i*3]
			nBr++
		case i%10 == 0:
			midSum += result.Pix[i*3]
			nMid++
		default:
			bgSum += result.Pix[i*3]
			nBg++
		}
	}
	bg := bgSum / float32(nBg)
	mid := midSum / float32(nMid)
	br := brSum / float32(nBr)
	if mid < 0.10 {
		return fmt.Errorf("mid not lifted: %v (want > 0.10)", mid)
	}
	if br < 0.4 {
		return fmt.Errorf("bright not stretched: %v (want > 0.4)", br)
	}
	if bg > 0.18 {
		return fmt.Errorf("background leaked: %v (want < 0.18)", bg)
	}
	return nil
}

func testColorCalibration() error {
	const w, h = 32, 32
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	for i := 0; i < w*h; i++ {
		// Red cast: R mean 0.2, G mean 0.1, B mean 0.1
		f.Pix[i*3] = 0.2
		f.Pix[i*3+1] = 0.1
		f.Pix[i*3+2] = 0.1
	}
	result := calibrateColor(f)
	// Compute means after
	var sr, sg, sb float64
	for i := 0; i < len(result.Pix); i += 3 {
		sr += float64(result.Pix[i])
		sg += float64(result.Pix[i+1])
		sb += float64(result.Pix[i+2])
	}
	n := float64(len(result.Pix) / 3)
	mr, mg, mb := sr/n, sg/n, sb/n
	// After calibration, the three channel means should be closer together
	maxDiff := math.Max(math.Abs(mr-mg), math.Max(math.Abs(mg-mb), math.Abs(mr-mb)))
	if maxDiff > 0.02 {
		return fmt.Errorf("channels not neutralised: means %.3f %.3f %.3f", mr, mg, mb)
	}
	return nil
}

func testGaussianBlur() error {
	const w, h = 9, 9
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	f.Pix[(4*w+4)*3] = 1.0 // single bright pixel
	result := gaussianBlur(f, 1.5)
	// Neighbours should be positive, sum should be ~1
	var total float32
	for i := 0; i < len(result.Pix); i += 3 {
		total += result.Pix[i]
	}
	if math.Abs(float64(total-1)) > 0.01 {
		return fmt.Errorf("total energy %.4f want ~1", total)
	}
	neighbour := result.Pix[(4*w+5)*3]
	if neighbour <= 0 {
		return errors.New("neighbour not blurred")
	}
	if neighbour >= 0.5 {
		return fmt.Errorf("neighbour too bright: %v", neighbour)
	}
	return nil
}

func testUnsharpMask() error {
	const w, h = 16, 1
	f := &frame{Width: w, Height: h, Pix: make([]float32, w*h*3)}
	// Step edge at x=8
	for x := 0; x < w; x++ {
		v := float32(0.2)
		if x >= 8 {
			v = 0.6
		}
		f.Pix[x*3] = v
		f.Pix[x*3+1] = v
		f.Pix[x*3+2] = v
	}
	result := unsharpMask(f, 0.8, 1.5)
	// Edge contrast should be enhanced: value just after edge higher, value just before lower
	beforeEdge := result.Pix[7*3]
	afterEdge := result.Pix[8*3]
	if afterEdge-beforeEdge < 0.4 {
		return fmt.Errorf("edge contrast not enhanced: before=%v after=%v", beforeEdge, afterEdge)
	}
	// Values should be clipped to [0, 1]
	for i, v := range result.Pix {
		if v < 0 || v > 1.0001 {
			return fmt.Errorf("out-of-range value at %d: %v", i, v)
		}
	}
	return nil
}

// =========================================================================
// main
// =========================================================================

// resolveBuildDate returns the build date from the ldflags-injected variable,
// falling back to the VCS commit time embedded by Go, and finally to "unknown".
func resolveBuildDate() string {
	if buildDate != "" {
		return buildDate
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.time" && s.Value != "" {
				return s.Value
			}
		}
	}
	return "unknown"
}

// printBanner writes version, repo URL and build date to stderr so stdout
// stays clean for scripts that consume stack outputs.
func printBanner() {
	fmt.Fprintf(os.Stderr, "SkyStacker %s | %s | built %s\n",
		version, repoURL, resolveBuildDate())
}

func main() {
	printBanner()
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "skystacker:", err)
		fmt.Fprintln(os.Stderr, "Run 'skystacker --help' for usage.")
		os.Exit(2)
	}
	if opts.Help {
		printUsage(os.Stdout)
		return
	}
	if opts.Version {
		fmt.Println(version)
		return
	}
	if opts.TestDebayer != "" {
		if err := testFITSFileDebayer(opts.TestDebayer); err != nil {
			fmt.Fprintln(os.Stderr, "skystacker:", err)
			os.Exit(1)
		}
		return
	}
	if opts.SelfTest {
		if runSelfTests() > 0 {
			os.Exit(1)
		}
		return
	}
	err = runPipeline(context.Background(), runOptions{
		InputDir:             opts.InputDir,
		OutputDir:            opts.OutputDir,
		Workers:              opts.Workers,
		Recursive:            opts.Recursive,
		Overwrite:            opts.Overwrite,
		Register:             opts.Register,
		DarkFile:             opts.DarkFile,
		FlatFile:             opts.FlatFile,
		BiasFile:             opts.BiasFile,
		Method:               opts.Method,
		MaskBG:               opts.MaskBG,
		PostProcess:          opts.PostProcess,
		StackOnly:            opts.StackOnly,
		PostOnly:             opts.PostOnly,
		NoColorCalib:         opts.NoColorCalib,
		NoSharpen:            opts.NoSharpen,
		BGDegree:             opts.BGDegree,
		StretchFactor:        opts.StretchFactor,
		SharpenAmount:        opts.SharpenAmount,
		SharpenRadius:        opts.SharpenRadius,
		HDRAmount:            opts.HDRAmount,
		StarRadius:           opts.StarRadius,
		StarSigma:            opts.StarSigma,
		BgPercentile:         opts.BgPercentile,
		NoAutoStretch:        opts.NoAutoStretch,
		STFShadowsClip:       opts.STFShadowsClip,
		STFTargetBG:          opts.STFTargetBG,
		STFLinked:            opts.STFLinked,
		NoHDRBlend:           opts.NoHDRBlend,
		HDRBlendFactor:       opts.HDRBlendFactor,
		NoHDRMix:             opts.NoHDRMix,
		NoDenoise:            opts.NoDenoise,
		DenoiseStrength:      opts.DenoiseStrength,
		DenoiseRadius:        opts.DenoiseRadius,
		DenoiseChroma:        opts.DenoiseChroma,
		NoCurves:             opts.NoCurves,
		CurvesBlack:          opts.CurvesBlack,
		CurvesGamma:          opts.CurvesGamma,
		AddSpikes:            opts.AddSpikes,
		SpikesPercent:        opts.SpikesPercent,
		SpikesLength:         opts.SpikesLength,
		SpikesIntensity:      opts.SpikesIntensity,
		SpikesArms:           opts.SpikesArms,
		SpikesSoftness:       opts.SpikesSoftness,
		SpikesMinRatio:       opts.SpikesMinRatio,
		SpikesLengthSoftness: opts.SpikesLengthSoftness,
		SpikesLengthMinRatio: opts.SpikesLengthMinRatio,
		SpikesMinCount:       opts.SpikesMinCount,
		NoExif:               opts.NoExif,
		Clean:                opts.Clean,
		SigmaClipThreshold:   opts.SigmaClipThreshold,
		SigmaClipIters:       opts.SigmaClipIters,
		TileSize:             opts.TileSize,
		Scale:                opts.Scale,
		Force:                opts.Force,
		Normalize:            opts.Normalize,
		LocalNormalize:       opts.LocalNormalize,
		LNTileSize:           opts.LNTileSize,
		CreateTIFF:           opts.CreateTIFF,
		PostCA:               opts.PostCA,
		PostColor:            opts.PostColor,
		PostFlatten:          opts.PostFlatten,
		PostSharpen:          opts.PostSharpen,
		PostHDR:              opts.PostHDR,
		PostStretch:          opts.PostStretch,
		NoCache:              opts.NoCache,
		CacheDir:             opts.CacheDir,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "skystacker:", err)
		os.Exit(1)
	}
}
