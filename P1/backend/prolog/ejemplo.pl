% ========= Hechos basicos =========
sintoma(fiebre).
sintoma(tos).
sintoma(dolor_garganta).
sintoma(dolor_cabeza).
sintoma(fatiga).

peso_severidad(leve,1).
peso_severidad(moderado,2).
peso_severidad(severo,3).

enfermedad(resfriado_comun, tipo(viral), sistema(respiratorio)).
enfermedad(influenza, tipo(viral), sistema(respiratorio)).
enfermedad(migrana,   tipo(neurologico), sistema(nervioso)).

caracteriza(resfriado_comun, tos, 2).
caracteriza(resfriado_comun, dolor_garganta, 2).
caracteriza(resfriado_comun, fiebre, 1).
caracteriza(influenza, fiebre, 3).
caracteriza(influenza, tos, 2).
caracteriza(influenza, fatiga, 2).
caracteriza(migrana, dolor_cabeza, 3).
caracteriza(migrana, fatiga, 1).

trata(paracetamol,            [resfriado_comun, influenza, migrana]).
trata(ibuprofeno,             [resfriado_comun, migrana]).
trata(oseltamivir,            [influenza]).
trata(jarabe_dextrometorfano, [resfriado_comun]).

contraindicado_por_alergia(ibuprofeno, aines).
contraindicado_por_alergia(oseltamivir, oseltamivir_alergia).

contraindicado_por_cronico(ibuprofeno, hipertension_no_controlada).

% ========= Auxiliares de listas =========
member(E,[E|_]).
member(E,[_|T]):- member(E,T).

sum_list([],0).
sum_list([H|T],S):- sum_list(T,S1), S is H+S1.

length([],0).
length([_|T],N):- length(T,N1), N is N1+1.

append([],L,L).
append([H|T],L,[H|R]):- append(T,L,R).

% ========= Afinidad y trazas =========
% SintomasSeveridad: [(sintoma,severidad), ...]
afinidad(Enf, Sv, Afin, Reglas):-
  findall(W,
    ( member((S,Sev), Sv),
      caracteriza(Enf, S, Pw),
      peso_severidad(Sev, Pv),
      W is Pw * Pv ),
    Pesos),
  sum_list(Pesos, Suma),
  findall(Pmax, caracteriza(Enf,_,Pmax), Pmaxs),
  length(Pmaxs, N), (N=:=0 -> Max is 1 ; Max is 9*N),
  Raw is Suma / Max,
  Afin is round(Raw * 100),
  findall(rule(caracteriza(Enf,S,Pw), severidad(S,Sev)),
          (member((S,Sev),Sv), caracteriza(Enf,S,Pw)),
          Reglas).

% ========= Medicamento seguro (sin negacion) =========
medicamento_seguro(Enf, Alergias, Cronicos, Med):-
  trata(Med, Ens), member(Enf, Ens),
  no_contra_alergias(Med, Alergias),
  no_contra_cronicos(Med, Cronicos).

no_contra_alergias(_, []).
no_contra_alergias(Med, [A|T]):- contraindicado_por_alergia(Med, A), !, fail.
no_contra_alergias(Med, [_|T]):- no_contra_alergias(Med, T).

no_contra_cronicos(_, []).
no_contra_cronicos(Med, [C|T]):- contraindicado_por_cronico(Med, C), !, fail.
no_contra_cronicos(Med, [_|T]):- no_contra_cronicos(Med, T).

% ========= Urgencia =========
nivel_urgencia(Enf, Sv, Urg):-
  ( Enf = influenza, member((fiebre,severo), Sv)     -> Urg = 'Consulta medica inmediata sugerida'
  ; Enf = influenza, member((fiebre,moderado), Sv)   -> Urg = 'Observacion recomendada'
  ; Enf = migrana,  member((dolor_cabeza,severo),Sv) -> Urg = 'Observacion recomendada'
  ; Urg = 'Posible automanejo'
  ).

% ========= Consulta y ordenamiento =========
% consulta(Sv, Alerg, Cron, Ordenado) -> Ordenado es lista de res(Enf,Afin,Med,Ur)
consulta(Sv, Alerg, Cron, Ordenado):-
  findall(res(Enf,A,Med,U),
    ( enfermedad(Enf,_,_),
      afinidad(Enf, Sv, A, _),
      A > 0,
      (medicamento_seguro(Enf, Alerg, Cron, Med) -> true ; Med = ninguno),
      nivel_urgencia(Enf, Sv, U)
    ),
    Res),
  ordenar_por_afinidad(Res, Ordenado).

% Para iterar desde Go: consulta_item(...) da soluciones ya ordenadas
consulta_item(Sv, Alerg, Cron, Enf, A, Med, U):-
  consulta(Sv, Alerg, Cron, Ord),
  member(res(Enf,A,Med,U), Ord).

% Ordenamiento por afinidad (desc)
ordenar_por_afinidad([], []).
ordenar_por_afinidad([H|T], S):-
  particionar_por_afinidad(H, T, May, Men),
  ordenar_por_afinidad(May, Sm),
  ordenar_por_afinidad(Men, Sn),
  append(Sm, [H|Sn], S).

afin_de(res(_,A,_,_), A).

particionar_por_afinidad(_, [], [], []).
particionar_por_afinidad(P, [X|Xs], [X|May], Men):-
  afin_de(X, Ax), afin_de(P, Ap), Ax >= Ap, !,
  particionar_por_afinidad(P, Xs, May, Men).
particionar_por_afinidad(P, [X|Xs], May, [X|Men]):-
  particionar_por_afinidad(P, Xs, May, Men).
