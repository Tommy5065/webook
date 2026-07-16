package memory

import (
	"context"
	"fmt"
)

type SmsMemory struct {
}

func NewSmsMemory() *SmsMemory {
	return &SmsMemory{}
}

func (sm *SmsMemory) Send(ctx context.Context, args []string, phoneNumber string) error {
	fmt.Printf("code:%v,valid minute:%v\n", args[0], args[1])
	return fmt.Errorf("hahhahhahahha.")
}
