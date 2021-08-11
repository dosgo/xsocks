// +build windows,amd64

package winDivert

import (
	_ "embed"
	"os"
)


//go:embed WinDivert64.dll
var winDivert64Bin []byte;
//go:embed WinDivert64.sys
var winDivert64Sys []byte;

func init() {
	divertDll="WinDivert64.dll"
	divertSys="WinDivert64.sys";

	_,err:=os.Stat(divertDll)
	if err!=nil {
		os.WriteFile(divertDll,winDivert64Bin,os.ModePerm)
		os.WriteFile(divertSys,winDivert64Sys,os.ModePerm)
	}
	dllInit(divertDll);
}
