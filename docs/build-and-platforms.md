# Как это собирается и на чём проверено

Что нужно знать, чтобы получить из этого дерева работающий бинарь на каждой из
трёх платформ, и какие грабли уже собраны. Собственно архитектура — в
[architecture.md](architecture.md); здесь только сборка.

## Одна команда на каждый случай

| Что нужно | Команда |
|---|---|
| Разработка | `wails3 dev` |
| Бинарь для этой платформы | `wails3 task build` |
| `.app` / инсталлятор | `wails3 task package`, `darwin:package:dmg`, `windows:package`, `linux:package` |
| Headless-сервер | `wails3 task build:server` |
| Docker-образ сервера | `wails3 task build:docker`, `run:docker` |
| Привязки к Go после правки `App` | `wails3 task generate:bindings` |
| Кросс-образ для Linux и macOS | `wails3 task setup:docker` |
| Платное издание | к любой из них — `EDITION=lifetime` |

`Taskfile.yml` в корне и по `Taskfile.yml` на платформу в `build/`. Makefile
здесь нет — цели `*-wails3` жили в v2-приложении и не переезжали.

## Теги сборки

`EXTRA_TAGS` по умолчанию `json1,sqlite3,frontend`:

- `json1`, `sqlite3` — cgo-SQLite (`mattn/go-sqlite3`), но только если cgo
  включён; без cgo они ничего не выбирают, и SQLite — чистый Go (ниже);
- `frontend` — `go:embed` каталога `webapp/pack`;
- `gtk3` добавляется на Linux сам (`GTK_TAG` в `build/linux/Taskfile.yml`);
- `server` — headless-сборка, у неё свои файлы (`window_server.go` вместо
  `window_desktop.go`);
- `production` — релизная сборка; без него каталог данных `XCIII-dev`, поэтому
  дев-сборка никогда не трогает настоящую установку;
- `lifetime` — платное издание: лишние шаблоны досок и свой фид обновлений.
  Добавляется через `EDITION=lifetime`, а не руками: `EDITION=base` (по
  умолчанию) не добавляет тега вовсе. См. [editions.md](editions.md).

## Кеш сборки

`-gcflags=all=…` и `-trimpath` входят в ключ кеша каждого пакета, так что
сборка с другими флагами компилирует всё заново — tailscale и стандартную
библиотеку тоже; это около 100 секунд. Поэтому дев-сборка идёт с теми же
флагами, что и `go test` (`-gcflags=all="-l"`, который ставил шаблон Wails,
убран в 2026-09-15), и они делят кеш. Релиз отличается `-trimpath` и платит
за это одной холодной сборкой; тег `production` сам по себе дешёвый, он
меняет только пакеты с файлами под ним. Замеры — `build-speed-plan.md`.

## Два драйвера SQLite, и выбирает их cgo

Драйвер выбирается ограничением `sqlite3 && cgo`
(`server/services/store/sqlstore/sqlite.go`): тег и включённый cgo — это
`mattn/go-sqlite3`, всё остальное — `modernc.org/sqlite`, чистый Go
(`sqlite_pure.go`), зарегистрированный под тем же именем `sqlite3`, потому что
`DBType` — одновременно и имя диалекта, и имя драйвера. DSN пишется под тот же
выбор (`SQLiteDSN`): у двух драйверов разное написание `foreign_keys` и
`busy_timeout`, а незнакомый параметр оба молча игнорируют.

Раньше `CGO_ENABLED=0` был ловушкой: сборка с тегом `sqlite3` компилировалась
чисто, линковала заглушку mattn (`static_mock.go` — регистрирует `sqlite3` и на
каждый `Open` отвечает «Binary was compiled with 'CGO_ENABLED=0' […] This is a
stub») и умирала на первом запросе. Теперь то же сочетание получает modernc.

Миграции подчиняются тому же выбору. У golang-migrate два SQLite-пакета,
построчно одинаковых: они отличаются только драйвером, который импортируют
сбоку (`database/sqlite` — modernc, `database/sqlite3` — mattn). Какой взять,
решает то же ограничение (`sqliteMigrationDriver` в `sqlite.go` /
`sqlite_pure.go`), и в сборку попадает ровно одна SQLite. До 2026-09-15 миграции
всегда брали modernc, и cgo-сборка линковала обе: 18 с CPU на холодной сборке.

Без cgo не собирается только десктоп на macOS и Linux — из-за самого Wails (у
`wails/v3/pkg/mac` исключены все файлы, GTK на Linux), и падает он честно. На
Windows Wails cgo не нужен. Ещё одно место с cgo, `tailscale/certstore`,
откатывается на чистый Go.

## Платформы

Собрано и проверено с macOS/arm64, кросс для Linux — через
`wails3 task setup:docker`. Windows собрана и на самой Windows.

**macOS.** `build` / `package` / `darwin:package:dmg`: `.app` и `.dmg`
собираются, приложение запускается, вебвью грузит страницу с front door по
настоящему HTTP.

**Linux.** `wails3 task linux:build` даёт ELF, слинкованный с `libgtk-3`,
`libwebkit2gtk-4.1` и `libsqlite3`. Тег `gtk3` обязателен: по умолчанию Wails v3
берёт GTK 4, а его GTK-4 код требует 4.10 (`GtkFileDialog`), тогда как Debian
bookworm — и кросс-образ вместе с ним — на 4.8. Снять тег можно через
`GTK_TAG=`.

**Windows.** `wails3 task windows:build` даёт PE32+ x86-64 **без cgo**:
`CGO_ENABLED=0` задан в `build/windows/Taskfile.yml` жёстко, SQLite — modernc.
Ни MinGW, ни кросс-образа не нужно — и на самой Windows, и с мака, где это
тот же `go build` с `GOOS=windows`, а внутренний линкер Go честно ставит
GUI-подсистему.

Headless-сборка (`wails3 task build:server`) под Windows тоже без cgo:
`build:server` оставляет `CGO_ENABLED` на усмотрение Go, а без C-компилятора
это 0 и modernc. На Wails v3.0.0-beta.2, с которой проект начинался, она не
компилировалась вовсе — `webview_window_windows_nonclient.go` был помечен
только `windows`, без `!server`; апстрим исправил это в beta.4. Проверено на
beta.22: сервер стартует, прогоняет миграции на новой базе и отдаёт страницу.

Сборки с cgo под Windows больше нет вовсе — ни через MinGW, ни через
кросс-образ, — потому что cgo там нужен был одной SQLite. У того пути было две
грабли, и они — причина не возвращать его без нужды:

- build-скрипт кросс-образа экспортировал `CGO_CFLAGS="-w"`, из-за чего zig
  оставлял включённым UBSan и линковка падала на
  `undefined symbol: __ubsan_handle_*`; а zig-овый `lld-link` игнорирует
  `-H windowsgui`, так что `.exe` выходил с console-подсистемой и мигал
  консолью;
- gcc подкладывает в каждый exe свой манифест (`default-manifest.o`), и ld
  выбрасывает его молча, только если у него нейтральный язык. У WinLibs он
  собран с 0x409, поэтому линковка писала `.rsrc merge failure: multiple
  non-default manifests`, и в exe оставались оба манифеста под одним ключом.

`internal/procgroup` под Windows не компилировался вовсе — `Setpgid` и
`syscall.Kill` там не существуют. Разведён по файлам: `procgroup_unix.go` как
было и `procgroup_windows.go` (`taskkill /T`, затем `/T /F` после grace,
ожидание через `WaitForSingleObject`).

## Инсталляторы — только на своей платформе

Бинарники кросс-собираются, пакеты — нет: AppImage зовёт `ldd`, NSIS — это
`makensis`, ни того ни другого на маке нет.

## `webapp/pack` не должен исчезать ни на секунду

`go mod tidy` резолвит шаблон `go:embed all:` под всеми тегами и идёт
параллельно со сборкой фронтенда. Каталог держит открытым закоммиченный
`.gitkeep`; Vite чистит *вокруг* него, а не удаляет его (`emptyPack` в
`vite.config.ts`). Чистит именно сборка, а не Taskfile: там это был
`rm -rf`, а у shell-интерпретатора Task на Windows нет `rm`.

Контракт сборки, который обязан выполняться, чем бы фронтенд ни собирался:

- на выходе `webapp/pack/index.html` и `pack/static/*`;
- в `index.html` есть Go-шаблон `{{.BaseURL}}`;
- ссылки на ассеты из JS и CSS — относительные;
- работают `npm run pack`, `dev`, `deveditor`;
- `go build -tags frontend` встраивает `webapp/pack`, headless-сборка отдаёт
  страницу, `/api` и `/ws`;
- три глобала из README («What this app requires of the frontend») — каждый с
  feature-detection на месте вызова, потому что тот же бандл ездит в браузер;
- `webapp/bindings` совпадает с тем, что генерирует `wails3 task
  generate:bindings`, а `@wailsio/runtime` — ровно та версия Wails, что в
  `go.mod` (`bindings_test.go`, джоб `bindings` в CI).

## Разработка: страница из Vite

`wails3 dev` — шаблонный цикл Wails: Vite с HMR в фоне
(`common:dev:frontend`, порт `VITE_PORT`, по умолчанию 9245), приложение —
основным процессом. Адрес Vite приложение получает в `FRONTEND_DEVSERVER_URL`,
и dev-сборка берёт оттуда страницу и модули через front door, вставляя в HTML
тот же bootstrap, что и в релизе; `/api/v0` и `/ws` остаются доске
(`devserver.go`, `proxy.go`). HMR-сокет Vite открывается на origin страницы —
то есть на front door — и проходит тем же прокси. Релизная сборка
переменную не читает.

Переменную приложение не сбрасывает: preRun самого Wails по ней ждёт, пока Vite
ответит, прежде чем открыть окно, — это ровно то ожидание, которое здесь и
нужно. Но `localhost` в ней заменяется на `127.0.0.1`, и Vite запускается там
же (`--host 127.0.0.1`): `wails3 dev` всегда пишет `localhost`, Node на Windows
вешает его на `::1`, а на машине, где IPv6-loopback отказывает, Wails пять
секунд ждал уже поднятый Vite и ронял приложение (`devserver.go`).

## Сервер в контейнере и его база

`wails3 task build:docker` — образ серверного режима по шаблону Wails
(`build/docker/Dockerfile.server`): сначала собирается страница (`webapp/pack`
вшивается тегом `frontend`, node в образе нет), потом чистый Go без cgo на
`distroless/static`. `CGO_ENABLED=1` с glibc-образами даёт mattn вместо
modernc. `run:docker` запускает его с данными в томе `xciii-data`.

Настраивается окружением при `docker run`:

- `WAILS_SERVER_HOST`/`WAILS_SERVER_PORT` (или `SERVER_HOST`/`SERVER_PORT`) —
  где слушает front door; в образе `0.0.0.0:8080`;
- `XCIII_DB_TYPE` — `sqlite3` (по умолчанию), `postgres` или `mysql`, и
  `XCIII_DB_DSN` — строка подключения в синтаксисе драйвера; для SQLite это
  путь, по умолчанию файл в каталоге данных (`database_server.go`);
- `/data` — каталог установки (`XDG_CONFIG_HOME`): база SQLite, файлы,
  настройки. Без тома каждый перезапуск — новая установка.

На Postgres и MySQL работает доска; агенты и источники выключены и пишут в лог
почему (`appTablesSupported`): их SQL написан на диалекте SQLite. Что нужно,
чтобы это изменить, — в [deferred.md](deferred.md), «Одна база: хвосты плана».
Десктоп всегда на SQLite и ничего из этого не читает.

Образ — это доска. Агентам нужны CLI (claude, codex, git, node), которых в
distroless нет, так что серверу с агентами нужен `RUNTIME_IMAGE`, который их
несёт.

## Запакованное приложение — не потомок оболочки

launchd выдаёт `.app` ровно `PATH=/usr/bin:/bin:/usr/sbin:/sbin`, поэтому npx,
node, CLI агентов и плагин-исходник для него не существуют, — а дев-сборка,
запущенная из терминала, находит всё. `internal/userpath` спрашивает PATH у
оболочки входа при старте: node у менеджера версий лежит под номером версии,
который знают только rc-файлы пользователя. Чинится именно PATH процесса, а не
наши поиски: npx — это скрипт с `#!/usr/bin/env node`, и codex-адаптер запускает
codex CLI, так что «найти бинарь и запустить его с PATH от launchd» просто
сдвигает падение на один процесс дальше.

## Как это проверялось

Способ проверки — и его собственные ловушки — в [verifying.md](verifying.md).

Курлом по front door на обеих сборках: страница отдаётся, вызов привязки
(`POST /wails/runtime` с id метода из `webapp/bindings`) возвращает данные
настоящих реестров, `/ws` отвечает `101 Switching Protocols` с того же origin,
кросс-origin вызов биндинга получает `403`. В десктопной сборке WebKit-процесс
держит живое соединение с front door — страница загружена по HTTP, а не по
`wails://`.

Терминал: pty отдаёт вывод в окно и принимает ввод, `stty size` внутри
показывает размер, выставленный xterm-ом (то есть страница действительно
подключена), выход CLI закрывает сокет кодом 1000 и оставляет на карточке отчёт
с коммитом, попытка открыть тот же сокет с чужого origin — `403`.
