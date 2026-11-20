# language: es
Característica: Abono en efectivo (US-01b)
  Como cliente autenticado
  Quiero depositar efectivo en mi cuenta
  Para incrementar mi saldo y ver el nuevo saldo

  Antecedentes:
    Dado que el cajero ATM está operativo con 5000.00 de efectivo

  Regla: Todo abono válido incrementa el saldo y el efectivo del cajero

    Escenario: Abono exitoso
      Dado Alice tiene 234.56 en su cuenta
      Cuando Alice intenta abonar 100.00
      Entonces el abono es exitoso
      Y el saldo de Alice es 334.56
      Y el efectivo del cajero es 5100.00

    Escenario: Abono por la app admite centavos
      Dado Hamza tiene 198.76 en su cuenta
      Cuando Hamza intenta abonar 0.24 por la app
      Entonces el abono es exitoso
      Y el saldo de Hamza es 199.00

  Regla: El monto del abono debe ser válido

    Esquema del escenario: Abono inválido
      Dado Pedro tiene 300.00 en su cuenta
      Cuando Pedro intenta abonar <monto>
      Entonces la operación termina con error <codigo>
      Y el saldo de Pedro es 300.00

      Ejemplos:
        | monto  | codigo  |
        | -50.00 | E-MONTO |
        | 0.00   | E-MONTO |
        | 15.00  | E-DENOM |

    Escenario: No se abona a una cuenta inactiva
      Dado Pedro tiene 300.00 en su cuenta
      Y la cuenta de Pedro está inactiva
      Cuando Pedro intenta abonar 100.00
      Entonces la operación termina con error E-INACTIVA
