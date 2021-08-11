// +build windows,386

package winDivert

import (
	_ "embed"
	"os"
)


//go:embed WinDivert32.dll
var winDivert32Bin []byte;
//go:embed WinDivert32.sys
var winDivert32Sys []byte;



func init() {
	divertDll="WinDivert32.dll"
	divertSys="WinDivert32.sys";
	_,err:=os.Stat(divertDll)
	if err!=nil {
		os.WriteFile(divertDll,winDivert32Bin,os.ModePerm)
		os.WriteFile(divertSys,winDivert32Sys,os.ModePerm)
	}
	dllInit();
}
