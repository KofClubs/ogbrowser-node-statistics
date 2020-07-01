package server

import (
	"github.com/oschwald/geoip2-golang"
	"github.com/sirupsen/logrus"
	"net"
)

type Location struct {
	Latitude  float64
	Longitude float64
	Location  string
}

func GetLocation(ip string) (l Location, err error) {
	db, err := geoip2.Open("GeoLite2-City.mmdb")
	if err != nil {
		logrus.WithError(err).Error("failed to open ip db")
		return
	}
	defer db.Close()

	ips, _, err := net.SplitHostPort(ip)
	if err != nil {
		logrus.WithError(err).WithField("ip", ip).Error("failed to split ip address")
		return
	}

	ipd := net.ParseIP(ips)
	city, err := db.City(ipd)
	if err != nil {
		logrus.WithError(err).WithField("ip", ip).Error("failed to looking for city")
		return
	}

	location := ""
	if v, ok := city.City.Names["en"]; ok {
		location = v
	} else if v, ok := city.Country.Names["en"]; ok {
		location = v
	}
	if location == "" {
		logrus.WithField("ip", ipd).Info("unknown location for ip. use Default instead")
		return GetLocation("47.100.122.212:1000")
	}

	l = Location{
		Latitude:  city.Location.Latitude,
		Longitude: city.Location.Longitude,
		Location:  location,
	}
	return
}
