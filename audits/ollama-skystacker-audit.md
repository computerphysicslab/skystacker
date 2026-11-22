Based on the provided technical documentation and project scope, here's a structured analysis of the SkyStacker project and potential areas for assistance:

---

### **Key Project Components**
1. **Core Functionality**  
   - **Image Processing Pipeline**:  
     - **Stacking Methods**: Mean, median, sigma-clipping (MAD-based).  
     - **Calibration**: Dark, flat, bias frames; bad-pixel map (BPM) generation.  
     - **Post-Processing**: 8-step pipeline (background subtraction, denoise, HDR blending, etc.).  
   - **Astrometry & Alignment**:  
     - Plate-solving with `astrometry.net` (blind fallback).  
     - WCS reprojection for mosaic creation (blocked due to dependency on item 13).  
     - Gaia DR4 index tooling for narrow-field solutions (not yet accepted).  

2. **Technical Stack**  
   - **Language**: Go 1.27.4 (pure Go with first-party modules).  
   - **Dependencies**:  
     - ImageMagick (FITS decoding fallback).  
     - dcraw (CR3 support).  
     - MIT-licensed `go-qrcode` for terminal QR codes.  
   - **Data Formats**:  
     - Native support for JPG, PNG, FITS, SER, CR2/CR3.  
     - All-sky blind solver with quad-hash indexing (R&D phase).  

3. **Roadmap & Challenges**  
   - **Open Tasks**:  
     - **Native all-sky quad-hash solver** (item 73): Requires full-sky generation at fine nside.  
     - **WCS Mosaic** (item 27): Blocked until per-frame WCS solutions are available.  
   - **Unmet Acceptance Criteria**:  
     - Gaia DR4 index tooling (item 81) failed long-focal solve tests on M51/M33/M17/M8.  
     - Blind-density-cap and TETRA-style exponential keys need refinement.  

---

### **Potential Areas for Assistance**
1. **Debugging & Optimization**  
   - **Gaia DR4 Index Tooling**:  
     - Investigate why long-focal solves on M51/M33/M17/M8 are failing.  
     - Validate `--blind-region` and `--blind-allsky` parameters for regional index generation.  
   - **WCS Mosaic**:  
     - Resolve dependency on item 13 (likely per-frame WCS solutions).  
     - Implement cone-tile margin and regional selection tests for mosaic alignment.  

2. **Feature Implementation**  
   - **Native All-Sky Quad-Hash Solver**:  
     - Develop fine-nside full-sky generation for `--blind-allsky`.  
     - Enhance `--validate-blind-catalogs` for checksum validation of large indexes.  
   - **Calibration Improvements**:  
     - Optimize `--self-flat` for relative illumination maps using dithered lights.  
     - Refine BPM generation with median + MAD outlier detection.  

3. **Code & Architecture**  
   - **CLI Contract**:  
     - Clarify flag interactions (e.g., `--reference-crop` vs. `--stack-only`).  
     - Ensure config flags (`--config FILE`) override CLI flags correctly.  
   - **Performance**:  
     - Optimize memory/disk pre-flight checks for large datasets.  
     - Profile `--workers` scaling for parallel processing (NumCPU vs. custom limits).  

4. **Documentation & Testing**  
   - **Self-Tests**:  
     - Expand `runSelfTests` in `main.go` to cover edge cases (e.g., corrupted catalogs).  
     - Validate `--selftest` output against the 274 embedded test cases.  
   - **User Guides**:  
     - Document `--blind-density-cap`, TETRA-style keys, and quad diagnostics.  
     - Clarify distinctions between `--mosaic` and `--plate-solve` workflows.  

---

### **Next Steps**
1. **Prioritize Open Tasks**:  
   - Focus on item 73 (all-sky quad-hash solver) and item 27 (WCS mosaic) to unlock blocked features.  
2. **Validate Existing Workflows**:  
   - Re-test Gaia DR4 index tooling with updated density tests and quad diagnostics.  
3. **Community & Collaboration**:  
   - Share progress on `--blind-region` and `--blind-allsky` with the astrometry.net community.  
   - Collaborate on WCS mosaic implementation if item 13 dependencies are resolved.  

Let me know which specific area you'd like to dive into! 🌌
