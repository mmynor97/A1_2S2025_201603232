sintoma(fiebre).
sintoma(tos).
sintoma(dolor_garganta).
sintoma(dolor_cabeza).
sintoma(fatiga).

peso_severidad(leve, 1).
peso_severidad(moderado, 2).
peso_severidad(severo, 3).

enfermedad(resfriado_comun, tipo(viral), sistema(respiratorio)).
enfermedad(influenza, tipo(viral), sistema(respiratorio)).
enfermedad(migraña, tipo(neurologico), sistema(nervioso)).

caracteriza(resfriado_comun, tos, 2).
caracteriza(resfriado_comun, dolor_garganta, 2).
caracteriza(resfriado_comun, fiebre, 1).
caracteriza(influenza, fiebre, 3).
caracteriza(influenza, tos, 2).
caracteriza(influenza, fatiga, 2).
caracteriza(migraña, dolor_cabeza, 3).
caracteriza(migraña, fatiga, 1).

trata(paracetamol, [resfriado_comun, influenza, migraña]).
trata(ibuprofeno, [resfriado_comun, migraña]).
trata(oseltamivir, [influenza]).
trata(jarabe_dextrometorfano, [resfriado_comun]).

contraindicado_por_alergia(ibuprofeno, aines).
contraindicado_por_alergia(oseltamivir, oseltamivir_alergia).

contraindicado_por_cronico(ibuprofeno, hipertension_no_controlada).

member(E, [E|_]).
member(E, [_|T]) :- member(E, T).

incluye([], _).
incluye([H|T], L) :- member(H, L), incluye(T, L).

afinidad(Enf, SintomasSeveridad, Afinidad, Reglas) :-
    findall(W, (member((S, Sev), SintomasSeveridad),
                caracteriza(Enf, S, Pw),
                peso_severidad(Sev, Pv),
                W is Pw * Pv), Pesos),
    sum_list(Pesos, Suma),
    findall(Pmax, caracteriza(Enf, _, Pmax), Pmaxs),
    length(Pmaxs, N), (N =:= 0 -> Max is 1 ; Max is 3 * 3 * N),
    Raw is Suma / Max,
    Afinidad is round(Raw * 100),
    findall(rule(caracteriza(Enf,S,Pw), severidad(S,Sev)),
            (member((S,Sev), SintomasSeveridad), caracteriza(Enf,S,Pw)),
            Reglas).

sum_list([],0).
sum_list([H|T],S):- sum_list(T,S1), S is H+S1.

medicamento_seguro(Enf, Alergias, Cronicos, Med) :-
    trata(Med, Enferms), member(Enf, Enferms),
    \+ (member(A, Alergias), contraindicado_por_alergia(Med, A)),
    \+ (member(C, Cronicos), contraindicado_por_cronico(Med, C)).

nivel_urgencia(Enf, SintomasSeveridad, Urg) :-
    ( Enf = influenza, member((fiebre,severo), SintomasSeveridad) -> Urg = 'Consulta médica inmediata sugerida'
    ; Enf = influenza, member((fiebre,moderado), SintomasSeveridad) -> Urg = 'Observación recomendada'
    ; Enf = migraña, member((dolor_cabeza,severo), SintomasSeveridad) -> Urg = 'Observación recomendada'
    ; Urg = 'Posible automanejo'
    ).

consulta(Sv, Alerg, Cron, ResOrdenado) :-
    findall(Enf, enfermedad(Enf,_,_), Enferms),
    findall(r(Enf,Afin,Med,Ur,Regs),
        ( member(Enf, Enferms),
          afinidad(Enf, Sv, Afin, Regs),
          Afin > 0,
          ( medicamento_seguro(Enf, Alerg, Cron, Med) -> true ; Med = ninguno ),
          nivel_urgencia(Enf, Sv, Ur)
        ), Res),
    sort(2, @>=, Res, Ordenado),
    maplist(r_a_dict, Ordenado, ResOrdenado).

r_a_dict(r(E,A,M,U,Regs), _{enfermedad:E, afinidad:A, medicamento:M, urgencia:U, reglas:Regs}).

consulta_item(Sv, Alerg, Cron, Enf, Afin, Med, Ur) :-
    findall(r(EnfX,AfinX,MedX,UrX),
        ( enfermedad(EnfX,_,_),
          afinidad(EnfX, Sv, AfinX, _),
          AfinX > 0,
          ( medicamento_seguro(EnfX, Alerg, Cro_
