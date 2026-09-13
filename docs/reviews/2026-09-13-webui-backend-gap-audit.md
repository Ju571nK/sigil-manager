# 백엔드 개선에 따른 WebUI 갭 조사

조사일: 2026-09-13. 매니저 `047e777`, 백엔드 `origin/main` `626bf42` (v0.8.0).
마지막 계약 census 기준인 백엔드 `580d2b6` 이후의 변경과 현재 화면 구현을 비교했다.
백엔드 작업 브랜치의 추가 커밋 `a05e068`은 출시된 main 기능으로 간주하지 않았다.
소스 정적 검토 결과이며 실제 서버 연결·브라우저 실행 검증은 하지 않았다.

제품 역할은 WebUI이다. 탐지·수집·정책·등록 기능의 소유자는 `../sigil`이다.
현재 매니저에 존재하는 Go 계층은 인증·triage·읽기 API 중계 역할이며,
아래 Go 변경은 새 백엔드 기능 개발이 아니라 화면으로의 데이터 전달 보정이다.
이 조사에서는 기존 인증/triage 구조를 제거하거나 이전하지 않는다.

## 후속 진행

2026-09-13: 사용자 선택 항목 P1-1(controls)을 `codex/observed-controls`에서 구현했다.
호스트/이벤트 상세 표시, Go 중계 보존, low 항목 펼치기와 호환성 검증을 완료했다.
Go 테스트, 웹 단위 테스트 52개, 린트, 프로덕션 빌드, Playwright 22개가 통과했다.
이후 사용자 선택에 따라 P1-2~P1-5도 구현했다. 신규 사유 필드/키,
공유 이벤트 상세와 Hook 탐색, cursor 페이지네이션, 101번째 호스트 정책 조회,
Settings 및 상단 연결 오류 표시를 완료했다. 최종 검증은 Go 테스트,
웹 단위 테스트 59개, 린트/타입 검사/프로덕션 빌드, Playwright 32개 통과다.
브라우저 검증은 Mock Fleet 및 HTTP fixture를 사용했으며 실제 producer 배포에
접속하지 않았다. P2의 전체 범위 집계/에이전트 상태 탐색과 MDM은 후속 항목이다.
아래 본문은 조사 당시의 문제와 수용 기준을 보존한다.

## 우선순위

| 순서 | 항목 | 영향 / 완료 기준 | 백엔드 선행 작업 |
|---|---|---|---|
| P1-1 | 관측된 보안 설정(controls) 표시 | 호스트·이벤트 상세에서 설정명, 값, 출처, 관측 시각과 범위를 보여준다. 미보고/빈 목록/값 존재를 구분한다. | 없음; 기존 Go 중계 타입 보정 필요 |
| P1-2 | 신규 탐지 사유 11종과 기존 필드 누락 보완 | 어떤 MCP 서버·도구가 왜 문제인지, 어떤 파일·목적지·해시가 관련되는지 읽을 수 있다. | 없음 |
| P1-3 | 이벤트 상세와 Hook 조사 흐름 | Fleet/Host 이벤트에서 상세를 열고 Hook 결정·모드·세션·증거를 확인한다. | 없음 |
| P1-4 | 목록 페이지네이션과 범위 표시 | 101번째 이벤트/호스트에 접근하고 검색·개수의 조회 범위를 명시한다. | 현재 cursor 활용 가능 |
| P1-5 | Settings 연결/메타 오류 분리 | healthz 성공 + meta 실패를 정상 연결·무라이선스·서명 비활성으로 오표시하지 않는다. | 없음 |
| P2-1 | 범위별 자세·최신성 표시 | 호스트의 마지막 도구 평가와 프로젝트별 기록을 구분한다. 낮은 위험도도 사유를 펼쳐 볼 수 있다. | 완전한 현재 범위별 집계는 별도 API 협의 대상 |
| P2-2 | 에이전트 관측 상태 탐색 | Watcher/Sender/Stall 수치에서 관련 이벤트로 이동하고 과거 횟수와 현재 상태를 구분한다. | 기존 이벤트로 시작 가능 |
| 보류 | MDM 장치 상태 | 기존 #31/#211의 계약 확정 후 표시한다. | 필요 |

## P1-1: controls 데이터가 호스트 화면까지 전달되지 않음

백엔드 `ff982a3`, `6f6a296`, `789f3b6`는 로컬 Claude 관리 설정과 Codex
requirements의 제한 설정을 관측한다. `AiGuardControl`은 `id`, `source_path`,
`setting`, JSON `value`를 제공한다. 위험 점수는 변경하지 않는다.

- producer: `crates/sigil-core/src/event.rs`, `crates/sigil-server/src/fleet_index.rs`, `crates/sigil-server/src/routes/fleet_hosts.rs`.
- consumer: `internal/fleet/client.go::ToolAiGuard`에 controls가 없어 decode/re-encode 시 손실된다.
- `web/src/api/fleet.ts::ToolAiGuard` 및 `AiGuardEvidence`에 명시 타입이 없고 `AiGuardByTool`/`SlideOver`도 표시하지 않는다. 이벤트 원본 JSON에는 필드가 보존된다.

별도 “관측된 보안 설정” 영역을 만들고 absent/null은 미보고, []는 검사 결과 관측 없음으로 표시한다.
false 값을 누락하지 않고, 알 수 없는 ID도 읽을 수 있게 한다. 출처 파일과 평가 시각을 함께 표시한다.
“실제 차단됨”, “기업 정책 준수”, “안전 보장”으로 해석하거나 점수를 임의 차감하지 않는다.

검증 기준: producer fixture → Go → HTTP → TS → 화면에서 absent/null/[]/false/배열/unknown ID 보존.
기존 cache의 복사 동작도 새 slice/JSON 필드를 안전하게 처리하는지 확인한다.

## P1-2: 새 사유는 제목만 보이고 조사 정보가 누락됨

새 reason 11종과 필드 전체는 fleet 계약 §14.12에 기록했다.
MCP poisoning/hidden text/name shadow 3종, MCP baseline·schema·hint 4종,
Claude/Codex 설정 사유 4종이다. 백엔드의 `AiGuardReason`은 총 27종이다.

`web/src/components/ReasonList.tsx`는 `server_name`만 읽는다. 신규 `server`, `tool`,
`servers`, `text_kind`, `baseline_hash`, `current_hash`, `list`, `destination`,
`source`를 표시하지 않는다. `standing_command_approval.pattern`처럼 기존 필드가
맞는 경우는 이미 일부 보인다. unknown kind 때문에 파싱이 실패하는 문제는 아니다.

`reasonKey`도 신규 서버/도구/해시를 반영하지 않아 다른 MCP 사유들이 같은 React key를 갖는다.
필드 표시와 key를 함께 수정하고, 종류가 같은 여러 도구의 사유를 fixture로 검증한다.
기존 사유에서도 `script_path`, `hook_event`가 빠져 있다. 계약 §14.11의 잘못된 필드 기록도 정정했다.

MCP baseline은 최초 관측값이다. 새 도구를 “승인 위반 확정”, readOnlyHint 모순을
“실제 쓰기 수행 확인”으로 표현하지 않는다. 긴 해시/경로는 축약 표시와 전체 보기·복사를 제공한다.

## P1-3: Hook 렌더러는 있으나 정상 탐색 경로가 부족함

`SlideOver.tsx::HookFacts`에는 네 Hook 종류의 전용 렌더링이 이미 있다.
그러나 `useAlerts.ts`는 producer 기본 알림 정의만 조회하고 네 Hook 종류는 그 정의에 없다.
`EventsTable.tsx`는 행 클릭·이벤트 상세 링크가 없고 Tool도 `extractAiGuard`에만 의존한다.
따라서 Hook과 toggle drift의 Tool 열은 “—”이며 상세 증거를 탐색하기 어렵다.
Fleet Events의 눈에 보이는 종류 선택도 All/AI Guard 두 개뿐이다.

공유 이벤트 상세 패널과 URL 선택 상태를 추가한다. 목록 행에서 상세를 열고
kind/agent/host/time 필터로 이동하도록 한다. 기존 이벤트 단건 조회 API를 사용한다.
Hook allow/deny와 enforcement_mode를 함께 보여주고 action/session/tool_use ID와
hash, probe_error 등을 보충한다. 기본 알림 집계를 임의로 변경하지 않는다.

## P1-4: 첫 100건 제한이 조사와 상태 표시를 잘라냄

`useAlerts`, `useFleetEvents`, `useFleetRisk`, `useFleetCompliance`가 limit=100을
고정하고 next_cursor를 소비하지 않는다. Alerts의 상태·문자열 검색은 이 첫 페이지에만 적용된다.
`hosts/$hostId.tsx`는 첫 compliance 페이지에서 호스트를 찾아 그 이후 호스트에는 상태 pill이 없다.

더 보기 또는 페이지 이동을 제공하고, 필터 변경 시 cursor를 초기화한다.
폴링과 이전 페이지의 중복/누락 처리 방식을 정한다. “검색된 전체”와 “현재 불러온 범위”를 구분한다.
Controls 등 low/info 기록을 확인할 때 high-only 알림 조회를 재사용하지 않는다.
검증은 최소 101개 데이터, 반복 폴링, 필터 변경, 첫 페이지 밖 호스트로 수행한다.

## P1-5: Settings의 정상 상태 오표시

`settings.tsx`의 연결 판정은 healthz만 사용한다. producer healthz는 인증 없는
liveness로 meta/read API가 실패해도 성공할 수 있다. meta 로딩·실패 시에도
license는 “none (open-source server)”, auditSigningSummary(undefined)는 “disabled”가 된다.

liveness와 인증된 read API 상태를 분리하고 meta pending/error/성공을 명시한다.
성공한 응답에서 필드가 빠진 경우도 버전 호환성에 따른 “미보고”와 구분해야 한다.
감사 head null만으로 키 설정 유무나 서명 검증 성공을 확정하지 않는다.
401(중계 upstream_unauthorized), read API disabled, rebuild 503, 네트워크 실패와
기존 캐시 표시를 구분하고 재시도 동작을 제공한다.

## P2: 범위·최신성 및 에이전트 상태

`fleet_index_update.rs`는 `current_risk.insert(*tool, …)`로 최신 도구 평가를 덮어쓴다.
같은 도구의 user_global/project 전체를 합친 결과가 아니다. 기존 계약 §14.3/§14.4의 제한이다.
`AiGuardByTool`은 assessed_ts를 표시하지 않고 low 항목은 펼쳐도 점수만 보인다.
호스트 이벤트는 조회하지만 범위별 최신 평가로 재구성하지 않는다.

우선 최신 평가의 범위·시각·데이터 범위를 표시하고 low 항목도 같은 상세 컴포넌트를 제공한다.
이력 기반 범위 탐색은 retention/조회 범위를 명시한다. 완전한 실시간 범위 집계로 광고하지 않는다.

Watcher 복구 등 최근 backend 개선은 새 UI 관리 명령을 요구하지 않는다.
`PolicyHealthCard`의 Watcher/Sender/Stall 24h 수치를 관련 이벤트에 연결하는 것이 유용하다.
횟수가 남아 있다는 이유로 현재 장애라고 단정하거나 복구 이벤트 없이 “복구 완료”로 표시하지 않는다.

## 기존에 반영된 기능 / 후속 의존성

- toggle drift 전용 표시, 기존 4 Hook 렌더러, 도구 표시명, license 배너, audit head 요약은 이미 구현되어 있다. 새 기능으로 중복 개발하지 않는다.
- MDM은 [manager #31](https://github.com/Ju571nK/sigil-manager/issues/31) / [sigil #211](https://github.com/Ju571nK/sigil/issues/211)에서 진행할 별도 계약이다. 현재 fleet 응답에 device_context가 없다.
- 등록, mTLS 인증서 발급·바인딩, 서명 아티팩트 배포는 sigil 소유다. WebUI에 쓰기 동작이나 별도 구현을 추가하는 근거가 되지 않는다.
- scan remediation hints는 현재 fleet 계약의 구조화 필드가 아니다. CLI 내용을 API 기능으로 간주하지 않는다.
- 이번 조사는 새 producer API 차단 이슈를 만들지 않았다. 즉시 개선 항목은 기존 read API로 가능하며, 이미 알려진 범위별 집계 제한과 MDM 의존성은 별도로 명시했다.

## 권장 실행 순서

1. controls 중계/타입/호환성 fixture와 신규 ReasonList 표시를 함께 반영.
2. 공통 이벤트 상세와 Hook 필터·탐색을 연결.
3. cursor 페이지네이션, Settings 오류 상태, low/범위/최신성 표시 보완.
4. 기존 서버/최신 서버 fixture를 대상으로 호스트·이벤트·알림·설정 흐름을 검증.

이 문서 본문은 구현 전 조사 기록이며, 구현 및 검증 완료 범위는 위 “후속 진행” 절을 따른다.
