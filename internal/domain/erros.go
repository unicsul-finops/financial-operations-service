package domain

import "errors"

// fica separado para o handler conseguir responder 404 com errors.Is.
var ErrOperacaoNaoEncontrada = errors.New("operação não encontrada")
