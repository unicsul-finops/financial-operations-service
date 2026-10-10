package domain

import "errors"

// fica separado para o handler conseguir responder 404 com errors.Is.
var ErrOperacaoNaoEncontrada = errors.New("operação não encontrada")

// dois envios simultâneos do mesmo identificador: o segundo bate na constraint UNIQUE e o service devolve a operação já gravada.
var ErrOperacaoJaRegistrada = errors.New("operação já registrada")
