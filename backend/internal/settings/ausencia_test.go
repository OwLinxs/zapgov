package settings

import (
	"testing"
	"time"
)

// dt monta um instante local: 2026-06-DD HH:MM (junho/2026: dia 1 = segunda).
func dt(dia, hora, min int) time.Time {
	return time.Date(2026, 6, dia, hora, min, 0, 0, time.UTC)
}

func base() Ausencia {
	return Ausencia{
		Ativo:     true,
		FimSemana: "msg-fds",
		Feriado:   "msg-feriado",
		Feriados:  []string{"06-11", "2026-06-12"}, // recorrente e data única
		// Horário comercial: seg-sex 08:00–18:00.
		HorarioAtivo:  true,
		HorarioInicio: "08:00",
		HorarioFim:    "18:00",
		HorarioDias:   "1,2,3,4,5",
		HorarioMsg:    "msg-horario",
	}
}

func TestMensagemAusencia(t *testing.T) {
	cases := []struct {
		nome     string
		mod      func(a *Ausencia)
		quando   time.Time
		wantMsg  string
		wantSend bool
	}{
		// Dias úteis dentro do expediente → nada.
		{"segunda 10h dentro do horário", nil, dt(1, 10, 0), "", false},
		{"sexta 17:59 ainda dentro", nil, dt(5, 17, 59), "", false},

		// Fora do horário em dia útil.
		{"segunda 07:59 antes de abrir", nil, dt(1, 7, 59), "msg-horario", true},
		{"segunda 18:00 já fechou", nil, dt(1, 18, 0), "msg-horario", true},
		{"quarta 22h fechado", nil, dt(3, 22, 0), "msg-horario", true},

		// Fim de semana (6/jun = sábado, 7/jun = domingo).
		{"sábado de manhã", nil, dt(6, 10, 0), "msg-fds", true},
		{"domingo à noite", nil, dt(7, 21, 0), "msg-fds", true},

		// Feriados (11/jun = quinta, recorrente MM-DD; 12/jun = data única).
		{"feriado recorrente vence horário", nil, dt(11, 10, 0), "msg-feriado", true},
		{"feriado data única", nil, dt(12, 10, 0), "msg-feriado", true},

		// Toggles desligados.
		{"ausência off: sábado fica mudo", func(a *Ausencia) { a.Ativo = false }, dt(6, 10, 0), "", false},
		{"horário off: 22h em dia útil fica mudo", func(a *Ausencia) { a.HorarioAtivo = false }, dt(1, 22, 0), "", false},
		{"tudo off", func(a *Ausencia) { a.Ativo = false; a.HorarioAtivo = false }, dt(6, 10, 0), "", false},

		// Mensagens em branco não disparam.
		{"feriado sem msg cai no silêncio", func(a *Ausencia) { a.Feriado = "" }, dt(11, 10, 0), "", false},
		{"fds sem msg", func(a *Ausencia) { a.FimSemana = "" }, dt(6, 10, 0), "", false},

		// Sábado NÃO está nos dias de horário comercial → não manda msg-horario.
		{"sábado 22h sem ausência: dia não-útil fica mudo",
			func(a *Ausencia) { a.Ativo = false }, dt(6, 22, 0), "", false},

		// Sábado configurado como dia útil → horário passa a valer nele.
		{"sábado vira dia útil e fecha às 12h",
			func(a *Ausencia) { a.Ativo = false; a.HorarioDias = "1,2,3,4,5,6"; a.HorarioFim = "12:00" },
			dt(6, 14, 0), "msg-horario", true},
	}

	for _, tc := range cases {
		a := base()
		if tc.mod != nil {
			tc.mod(&a)
		}
		msg, send := MensagemAusencia(a, tc.quando)
		if msg != tc.wantMsg || send != tc.wantSend {
			t.Errorf("%s: MensagemAusencia(...) = (%q, %v), quer (%q, %v)",
				tc.nome, msg, send, tc.wantMsg, tc.wantSend)
		}
	}
}
