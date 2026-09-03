module gvpn

go 1.26.3

replace gvpn/server => ./server

require gvpn/server v0.0.0-00010101000000-000000000000

require (
	github.com/songgao/water v0.0.0-20200317203138-2b4b6d7c09d8 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
