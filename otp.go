package main

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type OTP struct {
	Key     string
	Created time.Time
}

type RetentionMap map[string]OTP

// NewRetentionMap creará un nuevo mapa de retención e iniciará la retención dado el período establecido
func NewRetentionMap(ctx context.Context, retentionPeriod time.Duration) RetentionMap {
	rm := make(RetentionMap)

	go rm.Retention(ctx, retentionPeriod)

	return rm
}

// NewOTP crea y agrega un nuevo otp al mapa
func (rm RetentionMap) NewOTP() OTP {
	o := OTP{
		Key:     uuid.NewString(),
		Created: time.Now(),
	}

	rm[o.Key] = o
	return o
}

// VerifyOTP se asegurará de que exista una OTP y devolverá verdadero si es así
// También eliminará la clave para que no pueda ser reutilizada.
func (rm RetentionMap) VerifyOTP(otp string) bool {
	// Verificar que OTP exista
	if _, ok := rm[otp]; !ok {
		// otp no existe
		return false
	}
	delete(rm, otp)
	return true
}

// La retención asegurará que se eliminen las OTP antiguas
// Está rutina bloquea, así que ejecutar como una goroutine
func (rm RetentionMap) Retention(ctx context.Context, retentionPeriod time.Duration) {
	ticker := time.NewTicker(400 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			for _, otp := range rm {
				// Añadir retención a Created y comprueba si ha caducado
				if otp.Created.Add(retentionPeriod).Before(time.Now()) {
					delete(rm, otp.Key)
				}
			}
		case <-ctx.Done():
			return

		}
	}
}
