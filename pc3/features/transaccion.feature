# language: es
Característica: Integridad de la transacción (US-04)
  Como banco (servicio ATM)
  Quiero que cada operación sea atómica y quede registrada
  Para garantizar consistencia contable y auditoría

  Antecedentes:
    Dado que el cajero ATM está operativo con 5000.00 de efectivo

  Regla: Todo intento deja rastro en el historial (éxito, fallo y error)

    Escenario: Una transacción exitosa se registra con saldo antes y después
      Dado Alice tiene 234.56 en su cuenta
      Cuando Alice intenta retirar 200.00
      Entonces la última transacción de Alice es RETIRO EXITOSA
      Y la transacción registra saldo antes 234.56 y saldo después 34.56

    Escenario: Una transacción rechazada se registra sin cambiar el saldo
      Dado Hamza tiene 198.76 en su cuenta
      Cuando Hamza intenta retirar 200.00
      Entonces la última transacción de Hamza es RETIRO RECHAZADA
      Y la transacción registra saldo antes 198.76 y saldo después 198.76

    Escenario: Una transacción con error de validación se registra
      Dado Pedro tiene 300.00 en su cuenta
      Cuando Pedro intenta retirar -20.00
      Entonces la última transacción de Pedro es RETIRO ERROR
      Y el historial de Pedro tiene 1 transacción

    Escenario: Un abono rechazado por error queda auditado
      Dado Pedro tiene 300.00 en su cuenta
      Cuando Pedro intenta abonar 15.00
      Entonces la última transacción de Pedro es ABONO ERROR

  Regla: Las transacciones son atómicas, o se completa todo o no se hace nada (RN-03)

    Escenario: Si la persistencia falla no cambia ni el saldo ni el historial
      Dado Alice tiene 234.56 en su cuenta
      Y que la persistencia falla
      Cuando Alice intenta retirar 100.00
      Entonces la operación termina con error E-SISTEMA
      Y el saldo de Alice es 234.56
      Y el historial de Alice tiene 0 transacciones

    Escenario: El abono tampoco queda a medias si falla la persistencia
      Dado Alice tiene 234.56 en su cuenta
      Y que la persistencia falla
      Cuando Alice intenta abonar 100.00
      Entonces la operación termina con error E-SISTEMA
      Y el saldo de Alice es 234.56
      Y el efectivo del cajero es 5000.00

  Regla: El historial se exporta como JSON

    Escenario: Historial con éxito, fallo y error en formato JSON
      Dado Carlos tiene 500.00 en su cuenta
      Cuando Carlos intenta retirar 150.00
      Y Carlos intenta retirar 900.00
      Y Carlos intenta retirar -20.00
      Entonces el historial de Carlos es JSON válido con 3 registros
