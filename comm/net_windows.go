// +build windows

package comm

import (
	"github.com/yijunjun/route-table"
)


func GetGateway()string {
	table, err := routetable.NewRouteTable()
	if err != nil {
		panic(err.Error())
	}
	defer table.Close()

	rows, err := table.Routes()
	if err != nil {
		panic(err.Error())
	}
	for _, row := range rows {
		if(routetable.Inet_ntoa(row.ForwardDest, false)=="0.0.0.0"){
			return routetable.Inet_ntoa(row.ForwardNextHop, false);
		}
	}
	return "";
}
