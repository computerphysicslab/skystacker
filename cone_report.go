package main

import (
	"math"
	"time"
)

func coneCoverageAllows(truncated bool, centerRA, centerDec, coveredRad, ra, dec float64) bool {
	if !truncated {
		return true
	}
	return angularSeparation(centerRA, centerDec, ra, dec) <= coveredRad
}

func computeConeTruncationReport(opts *runOptions, sol *plateSolution) *coneTruncationReport {
	if opts == nil || opts.ConeCatalog == "" || sol == nil {
		return nil
	}
	service, ok := coneServices[opts.ConeCatalog]
	if !ok {
		return nil
	}
	result, err := coneSearchCachedDetailed(gaiaDefaultCacheDir(), opts.ConeCatalog, service, "",
		sol.WCS.CRVAL1*180/math.Pi, sol.WCS.CRVAL2*180/math.Pi, opts.ConeRadiusArcsec, 60*time.Second)
	if err != nil {
		return nil
	}
	return &coneTruncationReport{
		Truncated:    result.Truncated,
		Rows:         result.Rows,
		RadiusArcsec: result.CoveredRadiusArcsec,
		AreaDeg2:     result.CoveredAreaDeg2,
		Reason:       result.LimitReason,
	}
}
