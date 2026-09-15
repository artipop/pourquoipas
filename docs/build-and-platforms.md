# Как это собирается и на чём проверено

Что нужно знать, чтобы получить из этого дерева работающий бинарь на каждой из
трёх платформ, и какие грабли уже собраны. Собственно архитектура — в
[CLAUDE.md](../CLAUDE.md); здесь только сборка.

## Одна команда на каждый случай

| Что нужно | Команда |
|---|---|
| Разработка | `wails3 dev` |
| Бинарь для этой платформы | `wails3 task build` |
| `.app` / инсталлятор | `wails3 task package`, `darwin:package:dmg`, `windows:package`, `linux:package` |
| Headless-сервер | `wails3 task build:server` |
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
Нового в графе это не добавило: modernc и так был в каждой сборке — его тянет
sqlite-драйвер golang-migrate (`migrate.go`).

Без cgo не собирается только десктоп на macOS и Linux — из-за самого Wails (у
`wails/v3/pkg/mac` исключены все файлы, GTK на Linux), и падает он честно. На
Windows Wails cgo не нужен. Остальные два места с cgo —
`prometheus/client_golang` (сборщик памяти под Darwin) и `tailscale/certstore` —
откатываются на чистый Go.

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
- работают `npm run pack`, `watchdev`, `deveditor`;
- `go build -tags frontend` встраивает `webapp/pack`, headless-сборка отдаёт
  страницу, `/api` и `/ws`;
- три глобала из README («What this app requires of the frontend») — каждый с
  feature-detection на месте вызова, потому что тот же бандл ездит в браузер.

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

Курлом по front door на обеих сборках: страница отдаётся,
`/wails/runtime.js` отдаётся, `Call.ByName('main.App.…')` возвращает данные
настоящих реестров, `/ws` отвечает `101 Switching Protocols` с того же origin,
кросс-origin вызов биндинга получает `403`. В десктопной сборке WebKit-процесс
держит живое соединение с front door — страница загружена по HTTP, а не по
`wails://`.

Терминал: pty отдаёт вывод в окно и принимает ввод, `stty size` внутри
показывает размер, выставленный xterm-ом (то есть страница действительно
подключена), выход CLI закрывает сокет кодом 1000 и оставляет на карточке отчёт
с коммитом, попытка открыть тот же сокет с чужого origin — `403`.
