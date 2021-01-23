package comm

import (
	"fmt"
	"reflect"
)

func RestartRun(worker interface{}) {
	c := make(chan int, 100)
	v := reflect.ValueOf(worker)
	if v.Kind() != reflect.Func {
		return
	}
	c <- 1
	for {
		<-c
		runner(v, c)
	}
}

func runner(v reflect.Value, c chan int) {
	fmt.Println("Startting ..... ")
	defer func() {
		if exception := recover(); exception != nil {
			fmt.Printf("Panic: %v\n", exception)
			c <- 1
		}
	}()
	v.Call(nil)
}