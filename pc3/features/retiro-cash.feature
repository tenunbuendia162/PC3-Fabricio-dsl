# language: es
Característica: Retiro de efectivo
  Como cliente autenticado
  Quiero retirar efectivo indicando un monto
  Para disponer de mis ahorros y ver mi nuevo saldo

  Antecedentes:
    Dado que el cajero ATM está operativo con 5000.00 de efectivo

  Regla: Los clientes no pueden retirar más de su saldo (RN-01, RN-02)

    Escenario: Retiro exitoso dentro del saldo
      Dado Alice tiene 234.56 en su cuenta
      Cuando Alice intenta retirar 200.00
      Entonces el retiro es exitoso
      Y el saldo de Alice es 34.56

    Escenario: Retiro rechazado por exceder el saldo
      Dado Hamza tiene 198.76 en su cuenta
      Cuando Hamza intenta retirar 200.00
      Entonces el retiro es rechazado
      Y la operación termina con error E-SALDO
      Y el mensaje es "Saldo insuficiente. Intente con un monto menor"
      Y el saldo de Hamza es 198.76

    Escenario: Retiro del saldo exacto deja la cuenta en cero sin sobregiro
      Dado Alice tiene 200.00 en su cuenta
      Cuando Alice intenta retirar 200.00
      Entonces el retiro es exitoso
      Y el saldo de Alice es 0.00

  Regla: El monto debe ser válido (US-03)

    Esquema del escenario: Monto inválido o fuera de límites
      Dado Pedro tiene 3000.00 en su cuenta
      Cuando Pedro intenta retirar <monto>
      Entonces la operación termina con error <codigo>
      Y el saldo de Pedro es 3000.00

      Ejemplos:
        | monto   | codigo  |
        | -20.00  | E-MONTO |
        | 0.00    | E-MONTO |
        | 55.00   | E-DENOM |
        | 1500.00 | E-LIMITE |

    Escenario: El límite diario se acumula entre retiros
      Dado Carlos tiene 5000.00 en su cuenta
      Cuando Carlos intenta retirar 600.00
      Y Carlos intenta retirar 600.00
      Entonces la operación termina con error E-LIMITE
      Y el saldo de Carlos es 4400.00

    Escenario: La app acepta montos que no son múltiplo de billetes
      Dado María tiene 100.00 en su cuenta
      Cuando María intenta retirar 25.50 por la app
      Entonces el retiro es exitoso
      Y el saldo de María es 74.50

    Escenario: Cuenta inactiva no puede retirar
      Dado Pedro tiene 300.00 en su cuenta
      Y la cuenta de Pedro está inactiva
      Cuando Pedro intenta retirar 100.00
      Entonces la operación termina con error E-INACTIVA

  Regla: El cajero debe estar operativo y tener efectivo

    Escenario: Cajero sin efectivo suficiente rechaza el retiro
      Dado que el cajero ATM tiene solo 100.00 de efectivo
      Y Alice tiene 500.00 en su cuenta
      Cuando Alice intenta retirar 200.00
      Entonces el retiro es rechazado
      Y la operación termina con error E-CAJERO
      Y el saldo de Alice es 500.00

    Escenario: Cajero fuera de servicio
      Dado que el cajero ATM está fuera de servicio
      Y Alice tiene 500.00 en su cuenta
      Cuando Alice intenta retirar 100.00
      Entonces el retiro es rechazado
      Y la operación termina con error E-CAJERO-OFF
