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
			s.Record("TypeA", "", "2026-06-15", "127-0-0-1.sslip.io", "1.1.1.1")
			snap := s.Snapshot()
			Expect(snap.ByCountry).To(Equal([]xip.CountStat{{Key: xip.UnknownCountry, Count: 1}}))
		})

		It("is a no-op on a nil *Stats so disabled stats never panic", func() {
			var s *xip.Stats
			Expect(func() { s.Record("TypeA", "US", "2026-06-15", "127-0-0-1.sslip.io", "1.1.1.1") }).ToNot(Panic())
			Expect(s.Snapshot().Total).To(Equal(uint64(0)))
			Expect(s.RecentRequests("", "", 10)).To(BeNil())
		})
	})

	Describe("RecentRequests()", func() {
		It("returns recent requests newest-first, filtered by type and country", func() {
			s := xip.NewStats()
			s.Record("TypeA", "US", "2026-06-15", "1-1-1-1.sslip.io", "9.9.9.1")
			s.Record("TypeAAAA", "GB", "2026-06-15", "--1.sslip.io", "9.9.9.2")
			s.Record("TypeA", "US", "2026-06-15", "2-2-2-2.sslip.io", "9.9.9.3")

			all := s.RecentRequests("", "", 10)
			Expect(all).To(HaveLen(3))
			Expect(all[0].Name).To(Equal("2-2-2-2.sslip.io")) // newest first

			onlyA := s.RecentRequests("TypeA", "", 10)
			Expect(onlyA).To(HaveLen(2))
			for _, e := range onlyA {
				Expect(e.Type).To(Equal("TypeA"))
			}

			gb := s.RecentRequests("", "GB", 10)
			Expect(gb).To(HaveLen(1))
			Expect(gb[0].IP).To(Equal("9.9.9.2"))
		})

		It("honors the limit", func() {
			s := xip.NewStats()
			for i := 0; i < 5; i++ {
				s.Record("TypeA", "US", "2026-06-15", "host.sslip.io", "9.9.9.9")
			}
			Expect(s.RecentRequests("", "", 2)).To(HaveLen(2))
		})
	})

	Describe("Save() and LoadStats()", func() {
		It("round-trips the counters through a file", func() {
			s := xip.NewStats()
			s.Record("TypeA", "US", "2026-06-15", "127-0-0-1.sslip.io", "1.1.1.1")
			s.Record("TypeNS", "DE", "2026-06-15", "example.com", "3.3.3.3")

			path := filepath.Join(GinkgoT().TempDir(), "stats.json")
			Expect(s.Save(path)).To(Succeed())

			loaded, err := xip.LoadStats(path)
			Expect(err).ToNot(HaveOccurred())
			Expect(loaded.Snapshot().Total).To(Equal(uint64(2)))
			Expect(loaded.Snapshot().ByType).To(ContainElement(xip.CountStat{Key: "TypeNS", Count: 1}))

			// recent requests survive the round-trip, newest-first
			recent := loaded.RecentRequests("", "", 10)
			Expect(recent).To(HaveLen(2))
			Expect(recent[0].Name).To(Equal("example.com"))
			Expect(recent[0].IP).To(Equal("3.3.3.3"))
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
