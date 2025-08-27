sintoma(fiebre).
sintoma(tos).
sintoma(dolor_garganta).
sintoma(dolor_cabeza).
sintoma(fatiga).
sintoma(diarrea).
sintoma(nausea).

peso_severidad(leve,1).
peso_severidad(moderado,2).
peso_severidad(severo,3).

enfermedad(resfriado_comun, tipo(viral), sistema(respiratorio)).
enfermedad(influenza, tipo(viral), sistema(respiratorio)).
enfermedad(migrana, tipo(neurologico), sistema(nervioso)).
enfermedad(sinusitis, tipo(bacteriano), sistema(respiratorio)).
enfermedad(gastroenteritis, tipo(viral), sistema(digestivo)).
enfermedad(asma, tipo(cronico), sistema(respiratorio)).
enfermedad(faringitis, tipo(bacteriano), sistema(respiratorio)).

caracteriza(resfriado_comun, tos, 2).
caracteriza(resfriado_comun, dolor_garganta, 2).
caracteriza(resfriado_comun, fiebre, 1).
caracteriza(influenza, fiebre, 3).
caracteriza(influenza, tos, 2).
caracteriza(influenza, fatiga, 2).
caracteriza(migrana, fatiga, 1).
caracteriza(migrana, dolor_cabeza, 3).
caracteriza(sinusitis, fiebre, 2).
caracteriza(sinusitis, dolor_cabeza, 2).
caracteriza(sinusitis, fatiga, 1).
caracteriza(gastroenteritis, fiebre, 1).
caracteriza(gastroenteritis, fatiga, 1).
caracteriza(gastroenteritis, diarrea, 3).
caracteriza(gastroenteritis, nausea, 2).
caracteriza(asma, fatiga, 1).
caracteriza(asma, tos, 2).
caracteriza(faringitis, dolor_garganta, 3).
caracteriza(faringitis, fiebre, 2).
caracteriza(faringitis, tos, 1).

trata(paracetamol, [resfriado_comun, influenza, migrana, sinusitis, gastroenteritis, faringitis]).
trata(ibuprofeno, [resfriado_comun, migrana]).
trata(oseltamivir, [influenza]).
trata(jarabe_dextrometorfano, [resfriado_comun]).
trata(amoxicilina, [sinusitis, faringitis]).
trata(rehidratacion_oral, [gastroenteritis]).
trata(salbutamol, [asma]).
trata(sumatriptan, [migrana]).

contraindicado_por_alergia(ibuprofeno, aines).
contraindicado_por_alergia(oseltamivir, oseltamivir_alergia).
contraindicado_por_alergia(ibuprofeno, desconocida).
contraindicado_por_alergia(ibuprofeno, desconocida).
contraindicado_por_alergia(ibuprofeno, desconocida).
contraindicado_por_alergia(ibuprofeno, desconocida).
contraindicado_por_alergia(ibuprofeno, desconocida).
contraindicado_por_alergia(ibuprofeno, desconocida).
contraindicado_por_cronico(ibuprofeno, hipertension_no_controlada).

% ==== Auxiliares de listas ====
member(E,[E|_]).
member(E,[_|T]):-member(E,T).

sum_list([],0).
sum_list([H|T],S):-sum_list(T,S1),S is H+S1.

length([],0).
length([_|T],N):-length(T,N1),N is N1+1.

append([],L,L).
append([H|T],L,[H|R]):-append(T,L,R).

% ==== Afinidad ====
afinidad(Enf,Sv,Afin,Regs):-
  findall(W,(member((S,Sev),Sv),caracteriza(Enf,S,Pw),peso_severidad(Sev,Pv),W is Pw*Pv),Pesos),
  sum_list(Pesos,Suma),
  findall(Pmax,caracteriza(Enf,_,Pmax),Pmaxs),
  length(Pmaxs,N),(N=:=0->Max is 1; Max is 9*N),
  Raw is Suma/Max,
  Afin is round(Raw*100),
  findall(rule(caracteriza(Enf,S,Pw),severidad(S,Sev)),
          (member((S,Sev),Sv),caracteriza(Enf,S,Pw)),Regs).

% ==== Medicamento seguro (sin negación \+) ====
medicamento_seguro(Enf,Als,Crs,Med):-
  trata(Med,Ens), member(Enf,Ens),
  no_contra_alergias(Med, Als),
  no_contra_cronicos(Med, Crs).

no_contra_alergias(_, []).
no_contra_alergias(Med, [A|T]):- contraindicado_por_alergia(Med, A), !, fail.
no_contra_alergias(Med, [_|T]):- no_contra_alergias(Med, T).

no_contra_cronicos(_, []).
no_contra_cronicos(Med, [C|T]):- contraindicado_por_cronico(Med, C), !, fail.
no_contra_cronicos(Med, [_|T]):- no_contra_cronicos(Med, T).

% ==== Urgencia ====
nivel_urgencia(Enf,Sv,U):-
  ( Enf=influenza, member((fiebre,severo),Sv) -> U='Consulta médica inmediata sugerida'
  ; Enf=influenza, member((fiebre,moderado),Sv) -> U='Observación recomendada'
  ; Enf=migrana, member((dolor_cabeza,severo),Sv) -> U='Observación recomendada'
  ; U='Posible automanejo'
  ).

% ==== Consulta principal y ordenamiento ====
consulta(Sv,Als,Crs,Ordenado):-
  findall(res(Enf,A,Med,U),
    ( enfermedad(Enf,_,_),
      afinidad(Enf,Sv,A,_),
      A>0,
      (medicamento_seguro(Enf,Als,Crs,Med)->true;Med=ninguno),
      nivel_urgencia(Enf,Sv,U)
    ),Res),
  ordenar_por_afinidad(Res,Ordenado).

consulta_item(Sv,Als,Crs,Enf,A,Med,U):-
  consulta(Sv,Als,Crs,Ord),
  member(res(Enf,A,Med,U),Ord).

ordenar_por_afinidad([],[]).
ordenar_por_afinidad([H|T],S):-
  particionar_por_afinidad(H,T,May,Men),
  ordenar_por_afinidad(May,Sm),
  ordenar_por_afinidad(Men,Sn),
  append(Sm,[H|Sn],S).

afin_de(res(_,A,_,_),A).
particionar_por_afinidad(_,[],[],[]).
particionar_por_afinidad(P,[X|Xs],[X|May],Men):-afin_de(X,Ax),afin_de(P,Ap),Ax>=Ap,!,particionar_por_afinidad(P,Xs,May,Men).
particionar_por_afinidad(P,[X|Xs],May,[X|Men]):-particionar_por_afinidad(P,Xs,May,Men).
