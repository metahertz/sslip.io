package xip_test

import (
	"os"
	"path/filepath"
	"xip/xip"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stats", func() {
	Describe("Record() and Snapshot()", func() {
		It("counts queries by type, country, and day", func() {
			s := xip.NewStats()
			s.Record("TypeA", "US", "2026-06-15")
			s.Record("TypeA", "US", "2026-06-15")
			s.Record("TypeAAAA", "GB", "2026-06-16")

			snap := s.Snapshot()
			Expect(snap.Total).To(Equal(uint64(3)))

			// by_type is sorted by count descending, so TypeA (2) comes first
			Expect(snap.ByType).To(HaveLen(2))
			Expect(snap.ByType[0]).To(Equal(xip.CountStat{Key: "TypeA", Count: 2}))
			Expect(snap.ByType[1]).To(Equal(xip.CountStat{Key: "TypeAAAA", Count: 1}))

			Expect(snap.ByCountry[0]).To(Equal(xip.CountStat{Key: "US", Count: 2}))

			// by_day is sorted chronologically
			Expect(snap.ByDay).To(Equal([]xip.CountStat{
				{Key: "2026-06-15", Count: 2},
				{Key: "2026-06-16", Count: 1},
			}))
		})

		It("buckets an empty country code as unknown (ZZ)", func() {
			s := xip.NewStats()
			s.Record("TypeA", "", "2026-06-15")
			snap := s.Snapshot()
			Expect(snap.ByCountry).To(Equal([]xip.CountStat{{Key: xip.UnknownCountry, Count: 1}}))
		})

		It("is a no-op on a nil *Stats so disabled stats never panic", func() {
			var s *xip.Stats
			Expect(func() { s.Record("TypeA", "US", "2026-06-15") }).ToNot(Panic())
			Expect(s.Snapshot().Total).To(Equal(uint64(0)))
		})
	})

	Describe("Save() and LoadStats()", func() {
		It("round-trips the counters through a file", func() {
			s := xip.NewStats()
			s.Record("TypeA", "US", "2026-06-15")
			s.Record("TypeNS", "DE", "2026-06-15")

			path := filepath.Join(GinkgoT().TempDir(), "stats.json")
			Expect(s.Save(path)).To(Succeed())

			loaded, err := xip.LoadStats(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(loaded.Snapshot().Total).To(Equal(uint64(2)))
			Expect(loaded.Snapshot().ByType).To(ContainElement(xip.CountStat{Key: "TypeNS", Count: 1}))
		})

		It("returns a fresh Stats when the file doesn't exist", func() {
			path := filepath.Join(GinkgoT().TempDir(), "does-not-exist.json")
			loaded, err := xip.LoadStats(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(loaded.Snapshot().Total).To(Equal(uint64(0)))
		})

		It("errors on malformed JSON", func() {
			path := filepath.Join(GinkgoT().TempDir(), "bad.json")
			Expect(os.WriteFile(path, []byte("{not json"), 0o644)).To(Succeed())
			_, err := xip.LoadStats(path)
			Expect(err).To(HaveOccurred())
		})
	})
})
