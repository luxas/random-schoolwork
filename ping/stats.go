package main

import (
	"math"
	"time"
)

type PingStats struct {
	startTime time.Time
	packets   []packetStat
}

type packetStat struct {
	successful bool
	rtt        *time.Duration
}

type PingSummary struct {
	NumPackets    uint64
	NumReceived   uint64
	TotalDuration time.Duration
	MinRTT        time.Duration
	AvgRTT        time.Duration
	MaxRTT        time.Duration
	SdevRTT       time.Duration
}

func (s *PingStats) Start() {
	s.startTime = time.Now()
}

func (s *PingStats) Calculate() *PingSummary {
	ps := &PingSummary{}

	if len(s.packets) == 0 {
		return ps
	}

	ps.TotalDuration = time.Since(s.startTime)

	rttsum := int64(0)
	for i, p := range s.packets {
		ps.NumPackets++
		if p.successful {
			ps.NumReceived++
		}
		if p.rtt == nil {
			continue
		}
		rttsum += p.rtt.Nanoseconds()
		if i == 0 {
			ps.MinRTT = *p.rtt
			ps.MaxRTT = *p.rtt
		} else {
			ps.MinRTT = processDurations(math.Min, ps.MinRTT, *p.rtt)
			ps.MaxRTT = processDurations(math.Max, ps.MaxRTT, *p.rtt)
		}
	}
	ps.AvgRTT = time.Duration(int64(rttsum / int64(len(s.packets))))

	rttdiffsum := float64(0)
	for _, p := range s.packets {
		if p.rtt == nil {
			continue
		}
		val := math.Pow(ms(*p.rtt)-ms(ps.AvgRTT), 2)
		rttdiffsum += val
	}
	sd := int64(math.Sqrt(rttdiffsum/float64(len(s.packets)-1)) * 1000000)
	ps.SdevRTT = time.Duration(sd)

	return ps
}

func (s *PingStats) PacketReceived(rtt time.Duration) {
	s.packets = append(s.packets, packetStat{
		successful: true,
		rtt:        &rtt,
	})
}

func (s *PingStats) PacketLost() {
	s.packets = append(s.packets, packetStat{
		successful: false,
	})
}

func ms(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1000000
}

func processDurations(fn func(float64, float64) float64, a, b time.Duration) time.Duration {
	return time.Duration(fn(float64(a.Nanoseconds()), float64(b.Nanoseconds())))
}
