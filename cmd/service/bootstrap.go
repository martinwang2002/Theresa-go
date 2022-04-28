package service

import (
	"go.uber.org/fx"

	"theresa-go/internal/appentry"
)

func Bootstrap() {
	opts := []fx.Option{}
	opts = append(opts, appentry.ProvideOptions(true)...)
	opts = append(opts, fx.Invoke(run))

	app := fx.New(opts...)

	app.Run()
}
