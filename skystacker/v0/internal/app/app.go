if err := os.MkdirAll(outputDir, 0o755); err != nil {
	return fmt.Errorf("create output directory: %w", err)
}

sumTiff := filepath.Join(outputDir, "stacked_sum.tiff")
sumJPG := filepath.Join(outputDir, "stacked_sum.jpg")
meanTiff := filepath.Join(outputDir, "stacked_mean.tiff")
meanJPG := filepath.Join(outputDir, "stacked_mean.jpg")
noCorrTiff := filepath.Join(outputDir, "stacked_no_correction_mean.tiff")
noCorrJPG := filepath.Join(outputDir, "stacked_no_correction_mean.jpg")
previewJPG := filepath.Join(outputDir, "stacked_preview.jpg")
reportTxt := filepath.Join(outputDir, "report.txt")

// --- Sum outputs (acc.Pix currently holds the raw sum) ---
logf("Writing sum image (TIFF)")
if err := writeFileAtomic(sumTiff, func(w io.Writer) error {
	return output.WriteFloat32RGB(w, refW, refH, acc.Pix)
}); err != nil {
	return fmt.Errorf("write sum tiff: %w", err)
}
logf("Writing sum preview (JPG)")
if err := writeFileAtomic(sumJPG, func(w io.Writer) error {
	return output.WriteAutoNormalizedPreviewJPEG(w, refW, refH, acc.Pix)
}); err != nil {
	return fmt.Errorf("write sum jpg: %w", err)
}

// --- Convert sum to mean in place ---
acc.Divide()

// --- Mean outputs ---
logf("Writing mean image (TIFF)")
if err := writeFileAtomic(meanTiff, func(w io.Writer) error {
	return output.WriteFloat32RGB(w, refW, refH, acc.Pix)
}); err != nil {
	return fmt.Errorf("write mean tiff: %w", err)
}
logf("Writing mean preview (JPG)")
if err := writeFileAtomic(meanJPG, func(w io.Writer) error {
	return output.WritePreviewJPEG(w, refW, refH, acc.Pix)
}); err != nil {
	return fmt.Errorf("write mean jpg: %w", err)
}

// --- No-correction mean outputs (identical to mean in v0.1) ---
logf("Writing no-correction mean image (TIFF)")
if err := writeFileAtomic(noCorrTiff, func(w io.Writer) error {
	return output.WriteFloat32RGB(w, refW, refH, acc.Pix)
}); err != nil {
	return fmt.Errorf("write no_correction_mean tiff: %w", err)
}
logf("Writing no-correction mean preview (JPG)")
if err := writeFileAtomic(noCorrJPG, func(w io.Writer) error {
	return output.WritePreviewJPEG(w, refW, refH, acc.Pix)
}); err != nil {
	return fmt.Errorf("write no_correction_mean jpg: %w", err)
}

// --- Legacy stacked_preview.jpg (kept for backward compatibility) ---
logf("Writing legacy preview (JPG)")
if err := writeFileAtomic(previewJPG, func(w io.Writer) error {
	return output.WritePreviewJPEG(w, refW, refH, acc.Pix)
}); err != nil {
	return fmt.Errorf("write preview: %w", err)
}

// --- Report ---
rep := &report.Report{
	Version:         Version,
	Timestamp:       start,
	InputDir:        inputDir,
	OutputDir:       outputDir,
	Discovered:      len(files),
	Accepted:        accepted,
	Rejected:        rejected,
	Width:           refW,
	Height:          refH,
	Channels:        3,
	Method:          "sum and arithmetic mean",
	AccumulatorType: "float32 (interleaved RGB)",
	OutputFormat:    "TIFF: uncompressed, little-endian, RGB, IEEE float32, single strip. JPG: 8-bit, gamma 0.5 display preview.",
	OutputScaling:   "sum.tiff: raw float32 sum of samples in [0,1]; mean.tiff / no_correction_mean.tiff: sum / N, range [0,1]; sum.jpg: auto-normalised by global max; mean.jpg / no_correction_mean.jpg / preview.jpg: gamma 0.5 of values in [0,1]. No numeric clipping in TIFF outputs.",
	OutputFiles: []string{
		sumTiff, sumJPG,
		meanTiff, meanJPG,
		noCorrTiff, noCorrJPG,
		previewJPG,
		reportTxt,
	},
	Workers:  workers,
	Duration: time.Since(start),
	Notes: []string{
		"No star registration or calibration was performed (v0.1).",
		"Frames were combined in their existing pixel coordinate system.",
		"stacked_no_correction_mean.tiff is byte-for-byte identical to stacked_mean.tiff in v0.1 because no registration or alignment is performed. It is produced so that future registered means (v0.2+) can be compared directly against the uncorrected stack.",
		"stacked_mean.jpg, stacked_no_correction_mean.jpg and stacked_preview.jpg are identical in v0.1 for the same reason.",
		"JPEG inputs are gamma-encoded (sRGB-like); samples were averaged in encoded space, not linear light.",
		"All JPG outputs are display previews, not scientific outputs.",
	},
}
if err := writeFileAtomic(reportTxt, func(w io.Writer) error {
	return rep.WriteTo(w)
}); err != nil {
	return fmt.Errorf("write report: %w", err)
}

logf("Done in %s -> %s", rep.Duration.Round(time.Millisecond), outputDir)
return nil
}