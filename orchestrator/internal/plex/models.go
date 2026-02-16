// Package plex provides integration with Plex Media Server XML API
package plex

import "encoding/xml"

// MediaContainer represents the root element of Plex XML responses
type MediaContainer struct {
	XMLName   xml.Name    `xml:"MediaContainer"`
	Size      int         `xml:"size,attr"`
	Video     []Video     `xml:"Video"`
	Directory []Directory `xml:"Directory"`
}

// Video represents a Plex video item (episode, movie, etc.)
type Video struct {
	RatingKey            string  `xml:"ratingKey,attr"`
	ParentRatingKey      string  `xml:"parentRatingKey,attr"`
	GrandparentRatingKey string  `xml:"grandparentRatingKey,attr"`
	Type                 string  `xml:"type,attr"`
	Title                string  `xml:"title,attr"`
	Index                int     `xml:"index,attr"`            // Episode number
	ParentIndex          int     `xml:"parentIndex,attr"`      // Season number
	GrandparentTitle     string  `xml:"grandparentTitle,attr"` // Series name
	Media                []Media `xml:"Media"`
}

// Media represents media information within a video
type Media struct {
	Part []Part `xml:"Part"`
}

// Part represents a file part of media
type Part struct {
	File string `xml:"file,attr"`
}

// Directory represents a Plex directory (season, show, etc.)
type Directory struct {
	RatingKey string `xml:"ratingKey,attr"`
	Type      string `xml:"type,attr"`
	Index     int    `xml:"index,attr"`
	Title     string `xml:"title,attr"`
}
