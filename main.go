package main

import "github.com/sasakiyori/k8s-csi-demo/app"

func main() {
	driver := app.NewDriver()
	driver.Run()
}
