# Meu Financeiro — Plano completo de implementação

> Aplicativo pessoal e familiar de finanças. Fundação → MVP 1 → MVP 2 → v1.1 → v1.2 → v1.3 → v2.0.
>
> Stack: **Go + SQLite + Svelte + Oracle Cloud Always Free**. Interface **dark fintech**, baseada na direção visual B escolhida na conversa.
>
> Consolidação: 5 de novembro de 2026. Documento de planejamento, preparado para ser versionado no repositório do projeto.

## Sumário

1. [Visão, escopo e decisões](#1-visão-escopo-e-decisões)
2. [Os 11 requisitos](#2-os-11-requisitos)
3. [Regras financeiras e fórmulas](#3-regras-financeiras-e-fórmulas)
4. [Roadmap e cronograma](#4-roadmap-e-cronograma)
5. [Fundação técnica](#5-fundação-técnica)
6. [MVP 1 — Controle essencial](#6-mvp-1--controle-essencial)
7. [MVP 2 — Casal e metas](#7-mvp-2--casal-e-metas)
8. [v1.1 — Cartões e CSV](#8-v11--cartões-e-csv)
9. [v1.2 — Combustível e financiamentos](#9-v12--combustível-e-financiamentos)
10. [v1.3 — Importação PDF](#10-v13--importação-pdf)
11. [v2.0 — Telegram e automações](#11-v20--telegram-e-automações)
12. [Modelagem de dados](#12-modelagem-de-dados)
13. [Arquitetura e API](#13-arquitetura-e-api)
14. [Design e experiência](#14-design-e-experiência)
15. [Requisitos não funcionais](#15-requisitos-não-funcionais)
16. [Segurança e privacidade](#16-segurança-e-privacidade)
17. [Infraestrutura, implantação e custo](#17-infraestrutura-implantação-e-custo)
18. [Backups e recuperação](#18-backups-e-recuperação)
19. [Testes e critérios de qualidade](#19-testes-e-critérios-de-qualidade)
20. [Prioridades, riscos e conclusão das entregas](#20-prioridades-riscos-e-conclusão-das-entregas)
21. [Referências e decisões pendentes](#21-referências-e-decisões-pendentes)

## 1. Visão, escopo e decisões

O Meu Financeiro será uma plataforma de gestão pessoal e familiar para duas pessoas, com contas individuais, visão da casa e controle explícito de privacidade. Começará com lançamentos manuais e evoluirá para cartões, importações, rateio, metas, combustível, financiamentos e Telegram.

O marco de uso cotidiano é o **MVP 2**: receitas e despesas confiáveis, relatórios, divisão proporcional dos gastos e metas de economia. As demais versões ampliam essa base.

### Decisões principais

- Monólito modular: uma API Go, um banco SQLite e uma aplicação Svelte.
- Backend com `net/http`; frontend com TypeScript, Vite, Tailwind CSS e componentes próprios.
- Nginx para HTTPS, arquivos estáticos e proxy; systemd para execução e reinício automático.
- Duas contas de acesso individuais vinculadas a um núcleo familiar.
- Valores em BRL, armazenados como inteiros em centavos; cálculos financeiros feitos no backend.
- Lançamentos manuais no início; CSV antes de PDF; Telegram na v2.0.
- Nenhuma dependência inicial de Open Banking, APIs de mapas, IA paga, Redis ou filas externas.
- SQLite em disco local, com WAL, migrações, integridade referencial e backups consistentes.
- Planejar os domínios futuros desde a Fundação, mas criar suas tabelas e módulos por migrações conforme cada entrega. Preparar a arquitetura não exige implementar tudo antecipadamente.
- Meta de custo recorrente de infraestrutura de R$ 0 dentro das cotas elegíveis da Oracle, sem tratar gratuidade como garantia de disponibilidade ou de capacidade.

As especificações abaixo preservam as decisões recuperadas da conversa e acrescentam detalhes operacionais e critérios de aceite para torná-las implementáveis. Tokens visuais, limites de upload e metas de recuperação apresentados como propostas podem ser calibrados na Fundação.

## 2. Os 11 requisitos

| ID | Requisito | Comportamento esperado | Entrega |
|---|---|---|---|
| RF01 | Entradas | Salários, extras, reembolsos e outras receitas com data, categoria, conta e titular | MVP 1 |
| RF02 | Despesas fixas | Água, luz, internet, aluguel e outras recorrências, com previsão, vencimento e pagamento | MVP 1 |
| RF03 | Despesas variáveis | Mercado, alimentação, saúde, lazer e demais gastos com categorias e pagador | MVP 1 |
| RF04 | Relatórios mensais gráficos | Receitas × despesas, categorias, comparação mensal e evolução financeira | MVP 1 |
| RF05 | Metas de economia e pessoais | Meta mensal e objetivos como viagem ou carro, individuais ou familiares | MVP 2 |
| RF06 | Cartões e importação | Cadastro de cartões; compras e parcelas; importação CSV/PDF; filtros por cartão ou consolidados | v1.1 CSV; v1.3 PDF |
| RF07 | Gastos compartilhados | Marcar despesas compartilháveis, aplicar rateio por renda e comparar contribuição devida e paga | MVP 2 |
| RF08 | Previsão de combustível | Veículos, rendimento, preço, trajetos salvos e frequência para estimar custo mensal | v1.2 |
| RF09 | Financiamentos | Parcelas pagas, saldo devedor, cronograma e próximo vencimento | v1.2 |
| RF10 | Dashboard consolidado | Saldos, resultado, próximos compromissos, gráficos, progresso e metas alcançadas | MVP 1, ampliado a cada versão |
| RF11 | Configurações | Nomes das duas pessoas, preferências, contas, categorias, privacidade e regras da casa | MVP 1; rateio no MVP 2 |

## 3. Regras financeiras e fórmulas

### 3.1 Dinheiro, datas e estados

- Armazenar R$ 317,40 como `31740`; nunca usar ponto flutuante binário para valores monetários.
- Receber valores monetários na API como centavos inteiros; exibir com formatação `pt-BR`.
- Guardar percentuais como razões inteiras ou pontos-base; definir uma única regra de arredondamento.
- Distinguir data da compra, competência, vencimento e data do pagamento.
- Guardar eventos técnicos em UTC e datas financeiras como datas civis. Configurar o fuso da família; o contexto inicial é `America/Fortaleza`.
- Usar estados previstos, pendentes, pagos e cancelados. “Vencido” pode ser derivado do vencimento e do saldo não pago.
- Transferências entre contas não são receitas nem despesas. Saldo inicial também não é receita do mês.
- Cancelamentos e estornos devem manter rastreabilidade. Após fechamento mensal, corrigir por ajuste ou reabertura explícita e auditada.

### 3.2 Despesa fixa não significa valor invariável

Separar **recorrência**, **valor previsto**, **valor efetivo** e **pagamento**.

Exemplo: energia prevista em R$ 280,00 e conta recebida de R$ 317,40. O orçamento mantém a referência de R$ 280,00; o realizado usa R$ 317,40. A diferença é R$ 37,40 acima do previsto.

A regra recorrente gera uma ocorrência por período. Confirmar o valor ou o pagamento atualiza essa ocorrência, sem criar outra despesa. Mudar uma recorrência altera somente ocorrências futuras elegíveis; o histórico pago permanece preservado.

### 3.3 Consumo, caixa e obrigações

Cada relatório deve declarar sua base:

| Visão | O que mede | Exemplo |
|---|---|---|
| Consumo/competência | Receitas e despesas reconhecidas no período | Parcela de compra atribuída à competência do mês |
| Fluxo de caixa | Dinheiro recebido ou pago pelas contas | Pagamento da fatura na data da saída bancária |
| Compromissos | Obrigações ainda não liquidadas | Fatura fechada com vencimento futuro |

Na v1.1, a política inicial de compras parceladas será reconhecer o consumo por parcela, na competência da fatura. Guardar também a compra original e seu valor total; a tela de compras pode mostrar esse total sem somá-lo novamente às parcelas nos relatórios.

**Compra no cartão e pagamento da fatura não são duas despesas.** Uma compra de R$ 200,00 gera consumo e obrigação no cartão. Pagar a fatura reduz a conta bancária e liquida a obrigação. No relatório de consumo, permanecem R$ 200,00; no caixa, aparece a saída no pagamento. Juros, IOF, multas e tarifas são despesas próprias, separadas do principal.

Uma transferência para quitar acerto entre o casal não é renda de quem recebe nem nova despesa familiar de quem paga. No consolidado, eliminar transferências internas.

### 3.4 Rateio proporcional por renda

Usar, por padrão, a **renda líquida mensal confirmada para rateio**, independente do total de receitas registradas. Extras não recalculam as proporções automaticamente.

Para cada pessoa `i`:

```text
R = r1 + r2
p_i = r_i / R
c_i = D × p_i
```

- `r_i`: renda de referência da pessoa no mês.
- `R`: renda combinada.
- `p_i`: percentual de responsabilidade.
- `D`: valor da despesa compartilhada.
- `c_i`: responsabilidade financeira da pessoa.

Exemplo: rendas de R$ 6.000,00 e R$ 4.000,00 geram proporções de 60% e 40%. No aluguel de R$ 2.000,00, as responsabilidades são R$ 1.200,00 e R$ 800,00. Se a primeira pessoa pagar tudo, a segunda tem R$ 800,00 a acertar.

Regras complementares:

1. Permitir 50/50 e percentuais personalizados, por mês e por despesa.
2. Exigir soma de 100% nas regras percentuais e soma exata do valor em centavos nas parcelas de responsabilidade.
3. Não aceitar renda de referência negativa. Renda zero de uma pessoa é válida se a renda combinada for positiva.
4. Se ambas as rendas forem zero ou estiverem ausentes, solicitar divisão manual ou usar uma regra padrão previamente aprovada; nunca dividir por zero.
5. Guardar a versão mensal da renda e copiar a regra efetiva para a despesa. Alterações futuras não modificam o histórico automaticamente.
6. Fixar o critério de arredondamento: calcular cotas racionais, truncar centavos e distribuir o restante pelos maiores resíduos; desempatar por ordem estável dos participantes.
7. Exemplo de arredondamento: R$ 0,01 dividido 50/50 resulta em R$ 0,01 e R$ 0,00. O total continua R$ 0,01.
8. Estorno de despesa compartilhada reverte as cotas originais, sem recalcular com a renda atual.

### 3.5 Quem pagou, quem deve e acertos

Registrar separadamente o pagador, os participantes, as cotas e os pagamentos efetivos. Permitir mais de um pagamento por despesa, inclusive por pessoas diferentes.

Para despesas compartilhadas já pagas:

```text
B_i = pagamentos elegíveis de i − responsabilidade de i
B_i ajustado = B_i + acertos enviados por i − acertos recebidos por i
```

- Saldo positivo: tem a receber.
- Saldo negativo: tem a pagar.
- Acerto sugerido entre duas pessoas: valor absoluto do saldo devedor, considerando apenas obrigações elegíveis já liquidadas e os acertos registrados.

Despesas pendentes mostram responsabilidades previstas, mas não geram reembolso antecipado. Em pagamento parcial, a parcela elegível para acerto é rateada proporcionalmente ao valor efetivamente pago. Consumo do cartão e desembolso do titular devem aparecer separados; o acerto realizado acompanha a liquidação da fatura e sua alocação às compras. Antecipar um reembolso exige registro explícito de adiantamento.

Exemplo consolidado, sem acertos anteriores:

| Indicador | Você | Parceira | Total |
|---|---:|---:|---:|
| Responsabilidade 60/40 | R$ 2.700,00 | R$ 1.800,00 | R$ 4.500,00 |
| Pagamentos realizados | R$ 3.300,00 | R$ 1.200,00 | R$ 4.500,00 |
| Saldo do acerto | +R$ 600,00 | −R$ 600,00 | R$ 0,00 |

A parceira transfere R$ 600,00. Depois do registro, ambos os saldos de acerto são zero, e as despesas familiares continuam R$ 4.500,00.

### 3.6 Indicadores do dashboard

```text
Resultado de consumo = receitas reconhecidas − despesas reconhecidas
Fluxo líquido de caixa = entradas externas de caixa − saídas externas de caixa
Saldo de conta = saldo inicial + entradas na conta − saídas da conta
Disponível planejado = saldo líquido das contas selecionadas
                       − compromissos de caixa em aberto no horizonte
                       − reservas ainda incluídas nesse saldo
Progresso da meta = valor reservado líquido / valor-alvo × 100
```

Transferências afetam saldos por conta, mas se anulam no consolidado quando ambas as contas são incluídas. Explicitar o horizonte do disponível planejado. Uma fatura em aberto já representa suas compras; não descontar compras e fatura novamente como compromissos distintos. Reservas em contas excluídas do saldo não são subtraídas outra vez.

“Economia mensal” deve indicar sua base: resultado positivo do período ou valor efetivamente reservado. São métricas diferentes. Patrimônio líquido registrado será ativos acompanhados menos dívidas acompanhadas; mostrar cobertura parcial quando faltarem investimentos ou outros bens. Limite de cartão não é saldo nem patrimônio.

### 3.7 Metas e aportes

Aporte é reserva de saldo ou transferência para conta de objetivo, não despesa adicional. Não permitir alocar o mesmo dinheiro integralmente a duas metas. Retiradas reduzem a reserva; o gasto financiado pela retirada é registrado uma única vez como despesa.

Exemplos:

- Economia efetiva de R$ 1.200,00 diante de meta mensal de R$ 1.500,00: **80%**.
- Viagem com R$ 3.500,00 reservados e alvo de R$ 10.000,00: **35%**.
- Necessidade mensal: `máximo(0, alvo − reservado) / meses restantes`, arredondada para cima em centavos. Prazo vencido exige revisão, sem divisão por zero.

## 4. Roadmap e cronograma

Estimativas para uma pessoa desenvolvendo em tempo parcial. São faixas de planejamento, não datas garantidas; dependem de disponibilidade, formatos bancários e validação com o casal.

| Fase | Duração | Janela ilustrativa no cenário mínimo | Resultado |
|---|---|---|---|
| Fundação | 1–2 semanas | Semana 1 | Autenticação, layout, banco, deploy e backup |
| MVP 1 | 3–4 semanas | Semanas 2–4 | Controle manual e relatórios |
| MVP 2 | 2–3 semanas | Semanas 5–6 | Rateio, acertos e metas |
| v1.1 | 3–4 semanas | Semanas 7–9 | Cartões, parcelas e CSV |
| v1.2 | 2–3 semanas | Semanas 10–11 | Combustível e financiamentos |
| v1.3 | 2–3 semanas | Semanas 12–13 | PDF com revisão |
| v2.0 | 1–2 semanas | Semana 14 | Bot Telegram e automações |

O resumo original estimou **14–20 semanas**. A soma aritmética das faixas detalhadas é **14–21 semanas**; esta é a faixa consolidada, antes de eventuais reservas adicionais. O MVP completo, até MVP 2, demanda aproximadamente **6–9 semanas**.

Dependências: Fundação → MVP 1 → MVP 2 → v1.1 → v1.2 → v1.3 → v2.0. PDF reutiliza a conciliação do CSV; Telegram reutiliza os serviços financeiros. Combustível e financiamentos são módulos independentes entre si, embora estejam agrupados na mesma versão.

Ao final de cada fase: demonstração com dados fictícios, testes dos critérios de aceite, correções, atualização da documentação e backup antes de implantar.

## 5. Fundação técnica

**Objetivo:** estabelecer uma base leve, segura e recuperável.

### Etapas

1. Criar repositório com `backend/`, `frontend/`, `docs/` e `deploy/`.
2. Configurar Go, Svelte, TypeScript, Vite e Tailwind; fixar versões e arquivos de dependências.
3. Criar configuração por ambiente e arquivo de exemplo sem segredos.
4. Preparar SQLite com WAL, `foreign_keys=ON` em cada conexão e `busy_timeout` limitado.
5. Implementar migrações versionadas e consultas tipadas, por exemplo com sqlc se o driver escolhido for compatível.
6. Criar usuários, família, associação, contas, categorias e preferências.
7. Implementar senha com hash, login, logout, sessão e autorização por recurso.
8. Construir layout responsivo, tokens visuais e componentes de formulário.
9. Preparar Nginx, HTTPS e serviço systemd com usuário próprio.
10. Criar rotina de backup e executar uma restauração em ambiente isolado.
11. Configurar CI com formatação, análise estática, testes e build.
12. Criar dados fictícios e contrato inicial da API.

**Entregáveis:** login funcional, dois perfis, núcleo familiar, nomes editáveis, API autenticada, migrações, layout desktop/mobile, implantação reproduzível e backup automatizado.

**Aceite:** cada pessoa acessa sua conta; pedidos sem autorização são negados; reiniciar o serviço preserva os dados; restaurar o backup recupera usuários, contas e relações.

## 6. MVP 1 — Controle essencial

**Objetivo:** controlar um mês completo sem depender de planilhas.

### Entradas — RF01

Campos: valor, data/competência, descrição, categoria, conta de destino, titular, recorrência opcional e estado de recebimento. Categorias iniciais: salário, trabalho extra, reembolso e outras receitas.

Reembolso vinculado a uma despesa pode ser tratado como redução daquela despesa conforme política explícita; não classificá-lo automaticamente como salário nem criar renda artificial em acertos internos.

### Despesas fixas — RF02

Cadastrar regra mensal, vencimento, valor previsto, categoria, conta e titular. Gerar ocorrências idempotentes, confirmar valor real e pagamento, listar contas pendentes e vencidas. Para vencimento no dia 31 em mês curto, usar o último dia válido; documentar essa política.

### Despesas variáveis — RF03

Criar, editar, cancelar e buscar gastos por data, categoria, conta e pagador. Categorias personalizáveis para mercado, alimentação, transporte, saúde e lazer. Diferenciar lançamento previsto de realizado.

### Relatórios e dashboard — RF04/RF10

- Receitas versus despesas por mês, com base identificada.
- Distribuição por categoria e evolução mensal.
- Saldo consolidado das contas selecionadas e resultado do mês em cartões distintos.
- Próximas despesas, transações recentes e comparativo previsto/realizado.
- Histórico do patrimônio registrado, com aviso de cobertura parcial quando aplicável.
- Filtros por período, conta, categoria, titular e estado.

Percentual de variação contra mês anterior zero deve mostrar “sem base comparável”, em vez de infinito. Meses sem dados devem ser reconhecíveis como tais.

### Configurações — RF11

Editar nomes e preferências, gerenciar contas/categorias, alterar senha, configurar fuso e ocultar valores. Sem cadastro público aberto; a segunda conta é provisionada por convite restrito ou operação administrativa.

**Aceite:** registrar receitas, contas recorrentes e despesas variáveis de um mês; conferir saldo por conta, total por categoria e resultado; editar sem duplicar movimentos; validar apresentação no celular.

## 7. MVP 2 — Casal e metas

**Objetivo:** concluir o MVP utilizável pelos dois.

### Nossa casa — RF07

1. Informar renda líquida de referência de cada pessoa por mês.
2. Confirmar regra proporcional, igualitária ou personalizada.
3. Marcar despesa como individual ou compartilhada.
4. Copiar a regra vigente para suas cotas, com edição explícita de exceções.
5. Registrar pagador e pagamentos, sem confundir desembolso com responsabilidade.
6. Exibir total da casa, cotas, pagamentos e acerto pendente.
7. Registrar transferência de acerto e comprovante textual opcional.
8. Fechar mês e preservar snapshots. Reabertura exige confirmação e registro de auditoria.

A tela “Finanças da parceira” só mostra detalhes pessoais autorizados. A família recebe os dados necessários das despesas compartilhadas, não acesso automático a todo o histórico individual.

### Metas — RF05

Criar meta mensal de economia e objetivos individuais/familiares com nome, alvo, prazo, conta/reserva, aportes e retiradas. Exibir progresso, valor restante e contribuição mensal necessária. Sinalizar metas concluídas sem limitar o valor efetivo a 100%; a barra pode saturar visualmente em 100% e mostrar excedente em texto.

**Aceite:** reproduzir os exemplos 60/40 e de R$ 600,00 de acerto; liquidar sem inflar receita/despesa; alterar renda futura sem alterar mês fechado; reservar para viagem sem gerar gasto duplicado.

## 8. v1.1 — Cartões e CSV

As compras de todos os cartões entram pela importação da fatura CSV, inclusive quando o usuário prepara o arquivo por conta própria. A próxima entrega é reconhecer créditos e estornos na prévia, descontá-los do saldo da fatura e ajustar o rateio de compras compartilhadas pelas cotas originais.

### Cadastro e operação — RF06

| Campo | Exemplo |
|---|---|
| Nome | Nubank Platinum |
| Titular | Você |
| Instituição | Nubank |
| Limite | R$ 8.000,00 |
| Fechamento | Dia 15 |
| Vencimento | Dia 22 |
| Estado | Ativo |

Não armazenar número completo, CVV nem senha. Apelido e últimos quatro dígitos opcionais são suficientes.

Implementar compras à vista e parceladas, categoria, titular, compartilhamento, faturas abertas/fechadas/pagas/vencidas, pagamentos parciais, créditos/estornos e limite disponível estimado. Uma fatura parcialmente paga continua em aberto; o vencimento depende do saldo remanescente.

A compra e suas parcelas são entidades relacionadas. Exemplo: R$ 900,00 em três vezes gera parcelas de R$ 300,00 em três faturas. Na política de competência por parcela, cada mês reconhece R$ 300,00; o relatório não soma R$ 900,00 da compra mais R$ 900,00 das parcelas.

Guardar datas reais das faturas, além dos dias-padrão do cartão. Fechamentos em fins de semana, ajustes do banco e compras no dia de corte exigem confirmação. Ao editar datas futuras, não reatribuir automaticamente faturas fechadas.

Limite utilizado segue a regra estimada do emissor, inclusive parcelas futuras quando aplicável; não prometer igualdade com o aplicativo do banco.

### Pipeline CSV

```text
Upload → validação → adaptação do formato → normalização
       → detecção de duplicidades → revisão/conciliação
       → confirmação → gravação atômica → relatório da importação
```

Etapas:

1. Selecionar cartão e fatura de destino.
2. Validar tamanho, codificação, delimitador e colunas.
3. Suportar um ou dois bancos utilizados pelo casal e um formato genérico configurável.
4. Normalizar datas, separadores decimais, descrições, sinais de crédito/débito e parcelas.
5. Exibir data, descrição, valor, categoria sugerida, número da parcela, alertas e ação por linha.
6. Permitir revisar as linhas e conciliar com itens já importados.
7. Mostrar total a importar e divergência com o total da fatura; considerar tarifas, créditos e pagamentos separadamente.
8. Confirmar lote com transação de banco e registrar resultado por linha.

Exemplo genérico:

```csv
data;descricao;valor;parcela;total_parcelas
2026-11-03;Supermercado;234,90;1;1
2026-11-04;Combustível;160,00;1;1
2026-11-05;Streaming;39,90;1;1
```

A convenção de sinal acima considera valor positivo como débito; cada adaptador deve declarar a sua. Não inferir sem revisão se o layout for ambíguo.

### Duplicidade e conciliação

- Guardar hash do arquivo por família/cartão e identificador externo por origem quando existir.
- Gerar candidatos por cartão, data, valor, descrição normalizada e parcela.
- Tratar compras iguais no mesmo dia como possíveis transações legítimas; sem ID estável, similaridade sugere conciliação, não exclusão silenciosa.
- Persistir vínculo entre linha importada e lançamento existente.
- Garantir idempotência de confirmação e reenvio do mesmo lote.
- Identificar cada parcela na fatura correspondente e preservar o vínculo ao reimportar o arquivo.

**Aceite:** reimportar o mesmo CSV preserva as compras e parcelas nas faturas corretas. Pagar a fatura gera uma única saída. O dashboard permite filtrar um cartão ou todos.

## 9. v1.2 — Combustível e financiamentos

### Combustível — RF08

Salvar por pessoa e mês o rendimento médio do veículo em km/L, o preço por litro e os trajetos com distância de ida, opção de incluir a volta e número de ocorrências no mês.

```text
km_mensais_rota = distância_por_trecho × trechos_por_ocorrência × ocorrências_no_mês
litros_rota = km_mensais_rota / rendimento_km_por_litro
custo_rota = litros_rota × preço_por_litro
custo_mensal = soma dos custos de todas as rotas
```

Exemplo: 20 km por trecho, ida e volta, 22 dias, 10 km/L e R$ 6,00/L:

```text
20 × 2 × 22 = 880 km/mês
880 / 10 = 88 litros/mês
88 × 6 = R$ 528,00/mês
```

Informar as ocorrências diretamente para o mês escolhido. Somar as distâncias de todos os trajetos e arredondar o custo mensal final em centavos. A previsão é um dado de planejamento pessoal.

### Financiamentos — RF09

Campos: nome, instituição, titularidade, principal original, saldo devedor, data de referência do saldo, juros quando conhecidos, método de amortização, número de parcelas, vencimentos e cronograma. Registrar pagamentos, atrasos e amortizações extraordinárias.

Dashboard do contrato: parcelas integralmente pagas, total de parcelas, percentual, próxima prestação ainda não quitada e saldo devedor com origem/data. Exemplo: **18/48 = 37,5%** das parcelas pagas; saldo ilustrativo de **R$ 32.500,00**. Percentual de parcelas não equivale a percentual de principal amortizado.

**Saldo devedor não é a soma das prestações restantes.** Cada prestação pode conter principal, juros, seguro e tarifas. Preferir cronograma oficial e saldo informado pela instituição. Sem dados suficientes, aceitar atualização manual identificada e não apresentar projeção como valor exato.

Para cronogramas compatíveis, a identidade básica é:

```text
saldo_final = saldo_inicial + encargos capitalizados − amortização do principal
prestação = amortização + juros + seguros + tarifas
```

Só oferecer simulação Price/SAC quando condições e hipóteses estiverem definidas; as regras contratuais prevalecem. Em amortização extra, registrar se reduz prazo ou prestação e atualizar o cronograma conforme confirmação do credor.

O crédito recebido de empréstimo aumenta caixa e passivo, sem ser receita de consumo. No pagamento, principal reduz dívida e juros/encargos são despesa; o caixa mostra a prestação inteira. Se o casal escolher ratear a prestação inteira, identificar isso como responsabilidade de caixa, separada do relatório de consumo.

**Aceite:** simulador retorna R$ 528,00 no exemplo; rotas múltiplas somam corretamente; contrato mostra 18/48, próximo vencimento e saldo com data; pagamento não duplica gasto; amortização usa fonte contratual ou atualização manual rastreável.

## 10. v1.3 — Importação PDF

Reutilizar exatamente o pipeline de revisão, conciliação e gravação do CSV.

| Tipo de arquivo | Tratamento |
|---|---|
| PDF com texto e layout suportado | Extração local e parser específico por instituição |
| PDF com texto e layout desconhecido | Extração assistida, edição e revisão manual |
| PDF digitalizado | Informar ausência de extração automática; OCR como evolução opcional |
| PDF protegido | Informar limitação e orientar envio de versão acessível; não guardar senha |

Usar, por exemplo, `pdftotext` do Poppler, executado com argumentos seguros, timeout e limites de recursos. Não enviar faturas a terceiros. Tratar conteúdo extraído como dado não confiável.

Limites iniciais propostos: **10 MB, 100 páginas e 30 segundos por extração**, ajustados após teste na VPS. Validar assinatura/tipo real, isolar diretório temporário, gerar nomes internos, impedir execução de conteúdo e eliminar arquivos após o processamento. Uma limpeza periódica remove resíduos de falhas ou reinícios.

Manter adaptadores versionados e amostras sintéticas ou anonimizadas. Validar totais e sinalizar ambiguidades. Não aceitar automaticamente uma extração só porque encontrou valores monetários.

**Aceite:** PDF suportado produz as mesmas entidades do CSV; compra já importada gera sugestão de conciliação; arquivo inválido não bloqueia o serviço; revisão precede qualquer gravação financeira; temporários são removidos.

## 11. v2.0 — Telegram e automações

O bot será outra interface para os mesmos serviços Go. Esta fase substitui sugestões preliminares de colocar Telegram em versões anteriores: o roadmap consolidado o mantém na **v2.0**, após cartões, combustível, financiamentos e PDF.

Comandos iniciais:

```text
/despesa 35.90 alimentação almoço
/receita 2500 freelance
/saldo
/resumo
/faturas
/metas
```

Definir conta/categoria padrão e pedir confirmação quando faltar dado essencial. Retornar descrição, valor, data, conta e identificador do lançamento. Desfazer deve gerar cancelamento auditável e validar titularidade.

### Integração

- Vincular conta do Telegram a usuário autenticado usando código de uso único, curto prazo e confirmação na web.
- Validar usuário e chat autorizado; restringir inicialmente a conversas privadas.
- Receber webhook HTTPS e validar `X-Telegram-Bot-Api-Secret-Token`.
- Registrar unicidade de `update_id` por bot para evitar reprocessamento.
- Persistir resultado e pendência de resposta; falha no envio da confirmação não deve recriar o lançamento.
- Reutilizar autorização, limites e regras financeiras da interface web.
- Permitir desvincular e revogar acesso.

Esses mecanismos de webhook e identificação de atualizações estão descritos na [Telegram Bot API](https://core.telegram.org/bots/api).

Começar com comandos estruturados e sem IA. Parser local de linguagem natural é evolução opcional; não é necessário para concluir a fase. Alertas de vencimento e resumo mensal podem usar tarefas idempotentes e preferências de horário/consentimento.

A meta é custo adicional zero para uso pessoal normal dentro dos limites aplicáveis. Domínio é opcional; usar subdomínio gratuito com HTTPS ou considerar long polling se não houver endpoint público apropriado. Não operar polling e webhook simultaneamente.

Textos enviados ao bot passam pelo Telegram: não solicitar senhas, CVV, número completo de cartão ou faturas pelo chat nesta versão.

**Aceite:** cada usuário registra e consulta apenas o que pode acessar; reenvio de update não duplica despesa; token inválido é rejeitado; indisponibilidade temporária da resposta não perde nem repete a operação.

## 12. Modelagem de dados

Modelo lógico organizado por domínios. As entidades poderão ser consolidadas fisicamente quando isso simplificar a implementação sem perder integridade.

| Domínio | Entidades principais | Responsabilidade |
|---|---|---|
| Identidade e família | `users`, `households`, `household_members`, `settings`, `sessions` | Login, associação e preferências |
| Contas | `accounts`, `account_owners` | Titularidade, contas bancárias, dinheiro e reservas |
| Financeiro | `transactions`, `transaction_entries`, `categories`, `payments`, `payment_allocations` | Eventos, movimentos, reconhecimento e liquidação |
| Recorrências | `recurring_rules`, `scheduled_transactions` | Regras e ocorrências por competência |
| Compartilhamento | `income_snapshots`, `expense_splits`, `settlements`, `monthly_closures` | Rendas, cotas, acertos e fechamento |
| Cartões | `credit_cards`, `card_purchases`, `card_installments`, `card_statements` | Compras, parcelas e obrigações |
| Importações | `imports`, `import_rows`, `import_matches` | Lotes, revisão, deduplicação e vínculos |
| Metas | `savings_goals`, `goal_contributions`, `monthly_savings_targets` | Reservas e objetivos |
| Combustível | `fuel_forecasts` | Trajetos e parâmetros de previsão por pessoa e mês |
| Financiamentos | `loans`, `loan_installments`, `loan_payments` | Contratos, cronogramas e amortizações |
| Integrações e auditoria | `telegram_links`, `integration_updates`, `outbox`, `audit_events` | Identidades externas, idempotência, respostas e histórico |

`payments` e `payment_allocations` são propostas para representar pagamentos parciais e a liquidação de várias obrigações. `outbox` pode ser uma tabela simples no SQLite, sem fila externa. Todas devem ser criadas apenas quando necessárias à fase.

### 12.1 Campos e relações essenciais

**Identidade:** usuário com nome, identificador de login, hash de senha e estado; família com nome, moeda e fuso; membro com função e permissões. Sessão referencia usuário e tem expiração/revogação.

**Conta:** família, nome, tipo, moeda, estado e proprietários. Valor inicial deve ser um movimento identificado de abertura, ou saldo-base com data; nunca receita implícita.

**Transação:** identificador, família, criador, tipo, descrição, categoria, valor em centavos, competência, data do evento, estado, visibilidade, origem e referência externa opcional. Origem: manual, recorrência, CSV, PDF, Telegram ou ajuste.

**Entradas financeiras:** movimentos imutáveis ou reversíveis ligados à transação e à conta patrimonial pertinente, com papel explícito: consumo, caixa ou liquidação de obrigação. A implementação pode adotar um razão simplificado de partidas balanceadas; nesse caso, toda operação contabilizada deve fechar com soma zero. Não somar indiscriminadamente linhas de diferentes papéis nos relatórios.

**Pagamento:** data, valor, pagador, conta de origem e destino/obrigação; alocações ligam o pagamento à despesa, parcela ou fatura liquidada. A soma das alocações não excede o pagamento; adiantamentos/créditos ficam explicitamente identificados.

**Recorrência:** frequência, dia, vigência, valor estimado e política de vencimento. Ocorrência tem chave única `(recurring_rule_id, competência)`.

**Snapshot de renda:** família, pessoa, mês, renda de referência, regra escolhida, confirmação e versão. Percentuais personalizados podem ser armazenados em pontos-base, totalizando `10000`.

**Cota:** despesa, participante, valor em centavos, regra e versão efetivamente aplicadas. Total de cotas é igual ao valor compartilhado.

**Acerto:** remetente, destinatário, valor, data e vínculo aos movimentos financeiros. Uma referência impede que registrar o acerto e registrar a transferência criem duas saídas.

**Cartão/fatura:** cartão pertence a um titular; fatura pertence a um cartão e contém período, fechamento, vencimento, saldo e pagamentos. Parcela tem compra original, número, total de parcelas, valor e fatura. Uma parcela gera somente um reconhecimento de consumo.

**Importação:** família, usuário, cartão, origem, hash, versão do parser, estado e totais. Linhas têm resultado normalizado, erros, decisão de revisão e referência à entidade financeira correspondente. Dados brutos sensíveis devem ter retenção mínima; arquivo original não é mantido por padrão.

**Meta:** titularidade, alvo positivo, prazo opcional e estratégia de reserva. Contribuições referenciam movimentação ou alocação de saldo; nenhuma gera uma segunda despesa.

**Financiamento:** principal, saldo e data-base, fonte do saldo, cronograma, taxa e método quando conhecidos. Pagamento deve vincular principal, juros e encargos ao mesmo evento de caixa.

### 12.2 Invariantes e índices

- Chaves únicas estáveis; escolher UUID/ULID ou inteiro consistente na Fundação.
- `household_id` em registros de domínio; validar associações para impedir vínculos entre famílias diferentes.
- Chaves estrangeiras e restrições `CHECK`; validações entre múltiplas linhas em transação de serviço.
- Campos obrigatórios e valores positivos por tipo de operação; usar tipo explícito de estorno/crédito em vez de sinais ambíguos.
- Unicidade por ocorrência recorrente, parcela de compra, ID externo confiável, update Telegram e chave de idempotência.
- Mesma descrição/data/valor, por si só, não é chave única de compra.
- Índices por família + competência/data, conta, categoria, cartão/fatura e estado/vencimento.
- Versionamento otimista para impedir sobrescrita silenciosa de edição concorrente.
- Edições financeiras, cotas e movimentos relacionados são gravados atomicamente.
- Exclusão de conta/categoria com histórico deve arquivar ou bloquear, não apagar em cascata os registros financeiros.
- Meses fechados têm versão e referências dos cálculos usados; consultas históricas não usam configurações atuais para reinterpretar cotas antigas.

### 12.3 Exemplo de operação compartilhada

```text
Despesa: aluguel de R$ 2.000,00
Transação de consumo: R$ 2.000,00
Cotas: Você R$ 1.200,00; Parceira R$ 800,00
Pagamento: Você, conta corrente, R$ 2.000,00
Acerto posterior: Parceira → Você, R$ 800,00

Consumo familiar final: R$ 2.000,00
Desembolso líquido: Você R$ 1.200,00; Parceira R$ 800,00
Acerto pendente final: R$ 0,00
```

## 13. Arquitetura e API

### 13.1 Visão de execução

```text
Navegador: Svelte + TypeScript
          │ HTTPS, mesma origem
          ▼
Nginx ─── frontend estático
  │ /api e /webhooks/telegram
  ▼
Go: handlers → serviços de domínio → repositórios
  │                    ▲
  │                    └── tarefas agendadas / Telegram
  ▼
SQLite em volume local persistente
  │
  └── backup consistente → criptografia → destino externo à VPS
```

O backend escuta em loopback ou socket local. O banco não é exposto pela rede. Node.js é ferramenta de build do frontend, sem necessidade de processo Node em produção.

### 13.2 Organização do repositório

```text
meu-financeiro/
├── backend/
│   ├── cmd/server/
│   ├── internal/
│   │   ├── auth/
│   │   ├── users/
│   │   ├── households/
│   │   ├── accounts/
│   │   ├── transactions/
│   │   ├── recurring/
│   │   ├── sharing/
│   │   ├── cards/
│   │   ├── imports/
│   │   ├── goals/
│   │   ├── fuel/
│   │   ├── loans/
│   │   ├── reporting/
│   │   ├── jobs/
│   │   ├── platform/
│   │   └── integrations/telegram/
│   ├── migrations/
│   └── tests/
├── frontend/
│   └── src/
│       ├── lib/components/
│       ├── lib/api/
│       ├── lib/design/
│       └── pages/
├── deploy/
├── docs/
└── PLANO_IMPLEMENTACAO.md
```

Cada módulo segue **handler HTTP → serviço de negócio → repositório**. Relatórios consultam fontes canônicas; frontend e bot não reimplementam regras de rateio ou saldo. Evitar abstrações sem uso concreto e dependências circulares.

### 13.3 Contrato de API proposto

Prefixo `/api/v1`; payloads JSON, datas ISO, centavos inteiros, paginação e erros estruturados. Exemplos de rotas:

| Área | Rotas indicativas |
|---|---|
| Sessões | `POST /sessions`, `DELETE /sessions/current`, `GET /me` |
| Preferências | `GET/PATCH /settings`, `GET/PATCH /households/current` |
| Cadastros | `/accounts`, `/categories`, `/recurring-rules` |
| Financeiro | `/transactions`, `/transactions/{id}/payments` |
| Compartilhamento | `/sharing/income-snapshots`, `/sharing/summary`, `/settlements` |
| Metas | `/goals`, `/goals/{id}/contributions`, `/monthly-savings-targets` |
| Cartões | `/cards`, `/cards/{id}/statements`, `/card-purchases` |
| Importação | `POST /imports`, `GET /imports/{id}`, `POST /imports/{id}/confirm` |
| Combustível | `/fuel/forecast/{month}` | 
| Financiamentos | `/loans`, `/loans/{id}/payments` |
| Relatórios | `/reports/monthly`, `/dashboard` |
| Telegram | `POST /webhooks/telegram` fora do prefixo, com autenticação própria |

Rotas são proposta de contrato, não endpoints já implementados. Publicar especificação OpenAPI à medida que cada versão for entregue.

Criação exemplificativa de despesa:

```json
{
  "type": "expense",
  "amount_cents": 200000,
  "competence_date": "2026-11-01",
  "description": "Aluguel",
  "category_id": "categoria-moradia",
  "visibility": "household",
  "sharing": {
    "mode": "monthly_income_ratio",
    "period": "2026-11"
  }
}
```

Pagamento é registrado separadamente ou por comando composto atômico que crie ambos. IDs e percentuais enviados pelo navegador nunca dispensam validação de autorização e recálculo no servidor.

### 13.4 SQLite e tarefas

WAL permite leitores e escritor avançarem simultaneamente, mas mantém um escritor por vez. Usar transações curtas e uma conexão/caminho de escrita controlado; ajustar pool de leitura por medição. O arquivo deve ficar no mesmo host do processo, sem compartilhamento de arquivo em rede. Essas restrições seguem a [documentação WAL do SQLite](https://www.sqlite.org/wal.html).

Executar extração de PDF fora da transação financeira. Iniciar com uma extração pesada por vez, com tempo máximo e estado persistido. Usar rotinas Go ou `systemd timers` para recorrências, vencimentos, limpeza e backups. Proteger tarefas com chave idempotente/controle de execução; reiniciar o processo não pode gerar ocorrências repetidas.

## 14. Design e experiência

### 14.1 Direção visual B — dark fintech analítico

Interface escura, sóbria e responsiva, com cartões discretos, números legíveis e gráficos úteis. Verde para indicadores positivos e coral/vermelho para despesas, sempre acompanhados de texto, ícones ou padrões; cor sozinha não comunica estado.

| Token | Valor inicial | Uso |
|---|---|---|
| Fundo | `#0B1220` | Base da aplicação |
| Superfície | `#172334` | Cartões e painéis |
| Primária | `#10B981` | Ações e evolução positiva |
| Despesa/alerta | `#F87171` | Gastos e atenção |
| Texto principal | `#F8FAFC` | Conteúdo e números |

Validar contraste de cada combinação, principalmente botões com fundo verde; a paleta isolada não garante acessibilidade. Fonte Inter local ou nativa do sistema, números tabulares, espaçamento em múltiplos de 4 px, raios de 8–12 px e ícones lineares consistentes. Tema claro e opção de seguir sistema podem vir depois do dark, reutilizando os mesmos tokens.

### 14.2 Navegação

Desktop com barra lateral:

- Visão geral.
- Transações.
- Contas e cartões.
- Nossa casa.
- Metas e planejamento, incluindo combustível e financiamentos quando entregues.
- Relatórios.
- Configurações.

Mobile com quatro destinos principais — visão geral, transações, nossa casa e planejamento — e menu para os demais. Ação de novo lançamento acessível em qualquer tela pertinente.

Seletor de contexto: **Minhas finanças**, **Finanças da parceira** quando autorizado e **Nossa casa**. Filtros de período, categorias, contas, cartões, titular e base do relatório devem permanecer visíveis. Alterar contexto não pode expor dados privados nem misturar totais incompatíveis.

### 14.3 Telas e fluxos prioritários

1. **Dashboard:** saldos, receitas, despesas, disponível planejado e até dois gráficos principais; transações recentes e próximos compromissos.
2. **Novo lançamento:** valor primeiro, depois tipo, categoria, conta e data; descrição opcional. No mobile, formulário compacto. No MVP 2, adicionar compartilhamento sem dificultar o caso individual.
3. **Transações:** agrupamento por data, busca, filtros, edição e estados pendente/recorrente/parcelado.
4. **Planejamento:** comparação com metas, orçamento previsto/realizado e evolução.
5. **Nossa casa:** regra mensal, cotas, pagamentos, acertos e liquidação.
6. **Cartões/importação:** seletor de cartão/fatura, revisão de linhas e confirmação explícita do lote.

Para cada tela prever estados vazio, carregando, erro, sucesso, dados parciais e ausência de permissão. Informar salvamento e oferecer desfazer quando seguro. Botão para ocultar valores; valores não devem permanecer expostos em notificações ou títulos.

### 14.4 Gráficos e acessibilidade

- Barras para receitas/despesas; barras ou rosca simples para categorias; linhas para evolução; barras de progresso para metas.
- Mostrar unidade, período, base, legenda e totais; oferecer tabela ou resumo textual equivalente.
- Buscar WCAG 2.2 AA: contraste de texto comum de pelo menos 4,5:1, foco visível, rótulos, teclado e mensagens de erro associadas aos campos.
- Usar SVG ou biblioteca pequena e carregamento por módulo; evitar pacotes grandes para poucos gráficos.
- Não usar só animação ou cor para mostrar mudanças; respeitar preferência por movimento reduzido.
- Manter contas e categorias em memória durante a sessão, invalidando após alterações e limpando no logout. Não guardar dados financeiros persistentes no navegador sem necessidade explícita.

## 15. Requisitos não funcionais

| Área | Meta e forma de validação |
|---|---|
| Responsividade | Desktop, tablet e smartphone; validar desde 360 px de largura |
| Performance | Meta de p95 inferior a 200 ms em endpoints comuns, com carga leve e massa de referência; medir no servidor, sem upload/processamento pesado |
| Memória | Buscar menos de 150 MB para Go + Nginx em repouso; PDF e backups têm orçamento próprio medido |
| Integridade | Operações financeiras atômicas e idempotência em reenvios |
| Privacidade | Lançamentos individuais privados por padrão; permissões aplicadas no servidor |
| Disponibilidade | Reinício automático, health check e procedimento de recuperação; sem SLA comercial presumido |
| Backup | Diário, criptografado, verificável e externo à VPS |
| Recuperação | RPO proposto de até 24 h; RTO alvo de 4 h após infraestrutura disponível; validar em ensaio |
| Observabilidade | Logs estruturados, status de tarefas, última cópia válida e erros sem conteúdo financeiro sensível |
| Manutenção | Migrações versionadas, dependências fixadas, build reproduzível e documentação |
| Exportação | Exportar dados próprios em formato aberto com autorização e proteção contra fórmulas em CSV |
| Custo | Implantar apenas recursos elegíveis; acompanhar uso de computação, disco, rede e backup |

Performance e memória são objetivos, não resultados já medidos. Definir uma massa de referência — proposta inicial de 50 mil lançamentos e dois usuários simultâneos — e registrar os resultados reais na Fundação/MVP 1. Testar importações separadamente. O RTO depende de capacidade de provisionamento na nuvem; falta de capacidade pode ultrapassá-lo.

## 16. Segurança e privacidade

### 16.1 Acesso

- Senhas com Argon2id, sal único e parâmetros calibrados no hardware; nunca texto puro ou hash rápido simples.
- Sessões opacas aleatórias, armazenamento seguro no servidor, expiração, revogação e rotação após login.
- Cookies `Secure`, `HttpOnly` e `SameSite`; proteção CSRF nos endpoints autenticados que alteram estado.
- Limite de tentativas de login com resposta genérica; evitar revelar existência de usuários.
- Autorização por família, titularidade e visibilidade em toda consulta e mutação, incluindo relatórios, downloads e importações.
- Recuperação de acesso restrita e documentada; provisionamento local/administrativo pode evitar dependência inicial de e-mail, com token de uso único e invalidação de sessões.

### 16.2 Aplicação e arquivos

- HTTPS obrigatório, cabeçalhos de segurança, política de conteúdo e escape de textos.
- Queries parametrizadas; validação de payload, tamanho, datas, enums e identificadores.
- Não renderizar descrição de importação como HTML confiável.
- Extração de PDF sem interpolação de comandos e com limites de processo; dependências atualizadas.
- Proteger exportações CSV contra células iniciadas por caracteres interpretáveis como fórmula; preservar valor original internamente.
- Paginação e limites contra consultas excessivas; impedir enumeração de IDs e acesso a anexos por URL previsível.
- Não expor mensagens internas de banco, caminhos ou segredos em erros da API.

### 16.3 Host, segredos e dados

- Serviço Go sem privilégios administrativos, diretório de dados restrito e firewall mínimo.
- SSH por chave e acesso administrativo limitado; só portas necessárias abertas.
- Token Telegram, chave de backup e demais segredos fora do repositório e dos logs.
- Rotacionar credenciais e manter chave de recuperação do backup em local separado.
- Banco, WAL, arquivos temporários e backups têm as mesmas exigências de proteção.
- SQLite padrão não fornece criptografia transparente por si só. Usar proteção do volume/host e backups criptografados; avaliar solução específica se criptografia do banco em repouso for requisito adicional.
- Logs registram IDs, códigos e duração; não senhas, tokens, descrições financeiras completas ou conteúdo de fatura.
- Auditoria de login, alterações de regra, pagamentos, fechamento/reabertura, importações e correções financeiras.
- Respeitar privacidade de despesas individuais também em agregados, gráficos e notificações.

## 17. Infraestrutura, implantação e custo

### 17.1 Oracle Cloud Always Free

Provisionar VPS elegível na região da conta, com arquitetura compatível com o binário Go e o extrator PDF. Conferir cotas efetivas no console antes de criar instância, volumes, IPs e serviços auxiliares. Não depender dos créditos temporários do trial para manter o aplicativo.

A documentação da Oracle prevê recursos Always Free e possibilidade de recuperar instâncias consideradas ociosas. Logo, dados e procedimento de restauração precisam existir fora da VPS. A elegibilidade e as regras atuais devem ser conferidas na [documentação Always Free](https://docs.oracle.com/en-us/iaas/Content/FreeTier/freetier_topic-Always_Free_Resources.htm) e na [FAQ do Free Tier](https://www.oracle.com/cloud/free/faq/).

Não fixar no projeto uma quantidade universal de CPU/RAM gratuita; usar as cotas realmente disponíveis para a conta e região. Não gerar carga artificial para evitar políticas de ociosidade.

| Componente | Estratégia de custo |
|---|---|
| Go, SQLite e frontend compilado | Sem licença recorrente obrigatória |
| VPS, disco e rede | R$ 0 se elegíveis e dentro das cotas |
| HTTPS | Certificado gratuito quando o domínio/subdomínio permitir |
| Domínio próprio | Opcional; pode ter custo anual |
| Backup externo | Destino existente ou gratuito dentro da capacidade; confirmar cotas e tráfego |
| CSV/PDF | Processamento local, sem API paga |
| Mapas | Sem contratação; distâncias manuais |
| Telegram | Sem IA externa; uso pessoal dentro dos limites aplicáveis |

Alertas de orçamento não devem ser tratados como bloqueio automático de cobrança. Revisar mensalmente recursos e uso; manter plano de exportação para outro host.

### 17.2 Implantação

1. Build e testes no ambiente de desenvolvimento/CI, preferencialmente fora da VPS pequena.
2. Gerar binário e frontend para a arquitetura alvo; guardar versão e checksum.
3. Enviar artefatos a diretório de release, separado do banco persistente.
4. Ativar manutenção breve quando a migração exigir.
5. Gerar e verificar backup antes de alterar o esquema.
6. Aplicar migrações com versão registrada.
7. Atualizar serviço e arquivos estáticos; recarregar Nginx quando necessário.
8. Validar health check, login, consulta de saldo e criação de lançamento fictício em ambiente apropriado.
9. Observar erros e consumo de disco; documentar release e resultado.

Configurar systemd com reinício em falha e limites compatíveis; rotacionar logs. Publicar endpoint de saúde sem informações sensíveis. Monitorar espaço livre, falhas de login, tarefas, expiração de certificado e idade do último backup válido.

### 17.3 Rollback

Manter o release anterior. Reverter apenas o binário se o esquema continuar compatível; migração destrutiva pode exigir restauração do backup em manutenção. Avaliar perda de alterações posteriores e reconciliar antes de restaurar. Nunca presumir que um downgrade de código desfaz a migração.

## 18. Backups e recuperação

### 18.1 Política proposta

- Backup diário consistente e cópia extra antes de migrações relevantes.
- Retenção inicial: 7 diários, 4 semanais e 6 mensais, ajustada ao tamanho e às cotas.
- Criptografia antes do envio e verificação de checksum.
- Pelo menos uma cópia externa à VPS; idealmente também em outro provedor ou armazenamento local controlado.
- Chave de recuperação guardada separadamente e acessível ao responsável.
- Incluir banco, versão do aplicativo, versão do esquema e instruções de recuperação; configurações sensíveis devem ter proteção própria.
- Excluir temporários e faturas originais sem necessidade de retenção.
- Confirmar upload antes de considerar a execução bem-sucedida; alertar se não houver cópia válida há mais de 24 horas.

### 18.2 Consistência do SQLite

Usar a **Online Backup API** ou ferramenta equivalente que a utilize para produzir snapshot consistente com o banco ativo. A [documentação de backup do SQLite](https://www.sqlite.org/backup.html) descreve esse mecanismo.

Não copiar somente o arquivo `.db` enquanto há escritas em WAL: transações confirmadas podem estar no WAL. Não montar um backup manual copiando arquivos de banco/WAL em momentos diferentes. Validar o snapshot em arquivo separado antes de enviar.

### 18.3 Restauração testada

1. Provisionar ambiente isolado e obter release compatível.
2. Baixar a cópia externa, verificar checksum e descriptografar.
3. Conferir versão do esquema e executar `PRAGMA integrity_check` e `PRAGMA foreign_key_check` no banco restaurado.
4. Instalar com proprietário e permissões corretos.
5. Iniciar sem webhooks ou jobs que possam produzir efeitos externos.
6. Conferir usuários, contagens, saldos, cotas, faturas, metas e último período fechado contra o manifesto da cópia.
7. Medir tempo de recuperação e documentar dados potencialmente perdidos desde a cópia.
8. Após validação, liberar acesso e reativar tarefas controladamente.

Executar ensaio antes do primeiro uso real, mensalmente no início e após mudanças relevantes no processo. Um arquivo salvo sem teste de restauração não comprova recuperabilidade.

## 19. Testes e critérios de qualidade

### 19.1 Estratégia

- **Unitários:** centavos, arredondamento, rateio, competências, recorrências, parcelas, combustível e metas.
- **Integração com SQLite real:** transações atômicas, migrações, restrições, autorização, idempotência e relatórios.
- **Parsers:** fixtures sintéticas/anonimizadas para cada CSV e PDF suportado, incluindo ambiguidades.
- **Ponta a ponta:** login, lançamento, pagamento, rateio, acerto, meta e importação com revisão.
- **Regressão financeira:** conjunto de meses conhecidos com saldos e totais esperados em consumo e caixa.
- **Operacionais:** reinício, disco cheio tratado sem corrupção, backup externo e restauração.
- **Performance:** endpoints comuns e picos de importação medidos separadamente.

### 19.2 Matriz obrigatória

| Cenário | Resultado esperado | Fase |
|---|---|---|
| Despesa de R$ 0,01 | Exatamente 1 centavo, sem perda | MVP 1 |
| Transferência entre contas | Saldo por conta muda; consumo familiar não muda | MVP 1 |
| Recorrência executada duas vezes | Uma ocorrência por competência | MVP 1 |
| Conta prevista R$ 280,00, real R$ 317,40 | Realizado R$ 317,40, sem segunda despesa | MVP 1 |
| Vencimento dia 31 em fevereiro | Último dia válido conforme política | MVP 1 |
| Mês anterior com total zero | Variação sem divisão por zero | MVP 1 |
| Gasto de R$ 100,00 em 60/40 | R$ 60,00 e R$ 40,00 | MVP 2 |
| R$ 0,01 em 50/50 | Cotas somam exatamente R$ 0,01 | MVP 2 |
| Uma pessoa paga tudo | Crédito igual à cota da outra | MVP 2 |
| Total compartilhado R$ 4.500,00 | Exemplo resulta em acerto de R$ 600,00 | MVP 2 |
| Acerto de R$ 600,00 registrado | Zera obrigação sem inflar receitas/despesas | MVP 2 |
| Pagamento parcial ou por duas pessoas | Alocações e acerto refletem caixa elegível | MVP 2 |
| Renda combinada zero | Pede regra explícita; não gera erro aritmético | MVP 2 |
| Alterar renda do mês seguinte | Mês fechado preservado | MVP 2 |
| Aporte de R$ 500,00 em meta | Reserva cresce; despesa não cresce | MVP 2 |
| Usuário consulta despesa privada alheia | Acesso negado em detalhe e relatórios | Todas |
| Compra R$ 900,00 em 3 vezes | Três parcelas de R$ 300,00, sem total duplicado | v1.1 |
| Compra e pagamento de fatura R$ 200,00 | Consumo total R$ 200,00; caixa na liquidação | v1.1 |
| Importar mesmo CSV duas vezes | Não cria novos lançamentos | v1.1 |
| Duas compras iguais legítimas | Não elimina automaticamente uma delas | v1.1 |
| Reenvio da confirmação do lote | Efeito financeiro único | v1.1 |
| Estorno de compra compartilhada | Reverte cotas originais | v1.1 |
| Combustível 20 km × 2 × 22, 10 km/L, R$ 6/L | R$ 528,00 estimados; nenhum gasto automático | v1.2 |
| Financiamento com 18 de 48 parcelas | 37,5% das parcelas; principal não inferido daí | v1.2 |
| Amortização extraordinária | Atualiza saldo/cronograma com fonte rastreável | v1.2 |
| PDF já importado via CSV | Sugere conciliação; não duplica consumo | v1.3 |
| PDF inválido, grande ou lento | Rejeição controlada e limpeza | v1.3 |
| Telegram repete `update_id` | Uma única operação | v2.0 |
| Webhook sem segredo correto | Rejeitado sem escrita | v2.0 |
| Falha após gravar, antes de responder ao bot | Reenvio não recria transação | v2.0 |
| Restaurar backup externo | Integridade, relações e saldos preservados | Fundação e operação |

### 19.3 CI e revisão

Em cada alteração relevante: formatação, análise estática, testes do domínio afetado e build de backend/frontend. Executar testes de integração e regressão financeira antes de release; considerar detector de corrida do Go nos componentes concorrentes. Dependências e ferramentas de extração devem ser verificadas periodicamente quanto a atualizações de segurança.

Nenhuma versão é considerada concluída apenas por ter telas prontas. Regras financeiras e critérios de aceite da fase devem passar, sem regressões no histórico.

## 20. Prioridades, riscos e conclusão das entregas

### 20.1 Ordem de prioridade

- **P0 — Confiabilidade financeira:** centavos, saldos, pagamentos, competências, transferências, cotas e ausência de duplicação.
- **P0 — Proteção e recuperação:** acesso por usuário, privacidade, backup externo e restauração.
- **P1 — Uso cotidiano:** lançamento rápido, contas recorrentes, dashboard e funcionamento mobile.
- **P1 — Gestão familiar:** rateio justo, acertos e metas visíveis.
- **P2 — Automação de entrada:** cartões e CSV; depois PDF.
- **P2 — Planejamento ampliado:** combustível e financiamentos na ordem do roadmap.
- **P3 — Conveniência:** Telegram, alertas e melhorias opcionais de interpretação.

Essas prioridades orientam correções e decisões de escopo; não alteram a sequência das sete fases.

### 20.2 Riscos e mitigação

| Risco | Mitigação |
|---|---|
| Expansão do escopo antes de uso real | Concluir MVP 2 e validar com o casal antes de evoluir |
| Duplicação entre compra, parcela, fatura e pagamento | Fontes canônicas e testes separados de consumo/caixa |
| Alteração retroativa de rateio | Snapshots mensais, cotas persistidas e fechamento |
| Parser bancário incorreto | Adaptadores limitados, versões e revisão humana |
| Falso positivo de duplicidade | Conciliar por evidência; não excluir por mera semelhança |
| PDF consome toda a VPS | Uma extração por vez, limites e execução isolada |
| Saldo devedor inexato | Cronograma oficial ou saldo manual com data/fonte |
| Perda ou recuperação da VPS pela nuvem | Backup externo e deploy reproduzível |
| Exposição de despesa individual | Autorização no servidor e testes também em agregados |
| Cobrança inesperada | Validar elegibilidade e monitorar recursos, não apenas alertas |

### 20.3 Fora do escopo inicial

Sincronização bancária automática, Open Banking, OCR genérico, mapas pagos, IA externa, investimentos avançados, contabilidade fiscal, múltiplas moedas e suporte comercial multiempresa. Podem ser avaliados depois, sem bloquear o aplicativo familiar.

### 20.4 Checklist de conclusão de cada fase

- [ ] Requisitos e critérios de aceite da fase atendidos.
- [ ] Fluxos completos com estados de erro, vazio e carregamento.
- [ ] Autorização e privacidade verificadas nos novos recursos.
- [ ] Cálculos, relatórios e idempotência testados quando aplicáveis.
- [ ] Migrações revisadas e compatibilidade de rollback documentada.
- [ ] Backup válido e procedimento de recuperação mantido.
- [ ] Uso mobile e acessibilidade básica conferidos.
- [ ] Deploy validado e versão identificada.
- [ ] Documentação e limitações conhecidas atualizadas.
- [ ] Demonstração com dados fictícios e validação do comportamento pelo casal.

## 21. Referências e decisões pendentes

### Origem do plano

Conversa **“Recomendar stack enxuta”**, incluindo a escolha da direção B, a solicitação dos 11 requisitos e o plano de fases posterior. A sequência consolidada é a do plano mais recente: Fundação, MVP 1, MVP 2, v1.1 cartões/CSV, v1.2 combustível/financiamentos, v1.3 PDF e v2.0 Telegram.

### Referências técnicas

- [SQLite — Write-Ahead Logging](https://www.sqlite.org/wal.html): concorrência e restrições operacionais do WAL.
- [SQLite — Online Backup API](https://www.sqlite.org/backup.html): cópia consistente com banco ativo.
- [Oracle — Always Free Resources](https://docs.oracle.com/en-us/iaas/Content/FreeTier/freetier_topic-Always_Free_Resources.htm): recursos elegíveis e regras de uso.
- [Oracle — Free Tier FAQ](https://www.oracle.com/cloud/free/faq/): condições e capacidade disponível.
- [Telegram — Bot API](https://core.telegram.org/bots/api): comandos, webhooks, segredo e identificadores de atualização.

As condições comerciais, cotas e versões devem ser verificadas novamente na implantação. As metas deste documento são escolhas de projeto; não equivalem a garantias dos provedores.

### Confirmar durante a Fundação

- [ ] Bancos e formatos de CSV usados pelo casal; obter amostras anonimizadas antes da v1.1.
- [ ] Instituições e layouts de PDF prioritários antes da v1.3.
- [ ] Contas pessoais/compartilhadas e política de visibilidade desejada.
- [ ] Renda líquida de referência, regra padrão de rateio e política de extras.
- [ ] Convenção de fechamento e faturas, especialmente compras no dia de corte.
- [ ] Origem do saldo e cronogramas dos financiamentos.
- [ ] Região, cotas efetivas e capacidade disponível na Oracle.
- [ ] Subdomínio/domínio, destino externo de backup e responsável pela chave de recuperação.
- [ ] Disponibilidade semanal de desenvolvimento para transformar faixas de esforço em datas.

Essas decisões refinam a implementação; a criação deste plano não provisiona recursos, não integra contas e não publica o aplicativo.
