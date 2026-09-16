# Что вынесено из форка

Журнал усушки: одна строка на удалённую подсистему. Ведётся, потому что
«мертво» — суждение о сегодняшнем дне, а не о завтрашнем: решение о командной
работе уже один раз перевернуло план (`docs/fork-shrink-plan.md`, слой 0), и
незачем гадать, какое перевернёт следующим.

**Как вернуть.** Каждое удаление — отдельный коммит, ничего не размазано, и
находится он по имени того, что удалили: `git log -S '<имя>'`. Хешей здесь
нет: ветку переписывали при слиянии, и записанные хеши перестали существовать.
`git revert` найденного коммита поднимает подсистему целиком: код, интерфейс
стора, моки, тесты и правку страницы, если она была парной. Если с тех пор
менялся `Store`, после отката нужен `go generate ./server/services/store` —
моки и `public_methods.go` собираются из интерфейса.

**Чего здесь нет.** Тут только то, что удалено. То, что решили **не** трогать
— логин, публичный шаринг, многокомандность, планировщик, `search` и
`getTeamUsers`, — записано в плане, там же и почему.

## Слой 1 — сервер. Страница ничего не замечала

| Подсистема | Почему было мертво |
| --- | --- |
| telemetry (сервис, TelemetryID в `system_settings`, `Telemetry`/`TelemetryID` в конфиге и клиент-конфиге, `LvlFBTelemetry`, `rudderlabs/analytics-go`) | ключи Rudder — литералы `placeholder_*`, `getRudderConfig` возвращает пустой конфиг, если нет `RUDDER_KEY`/`RUDDER_DATAPLANE_URL`, а их никто не ставит |
| webhook (клиент, 8 вызовов в `app/`, `WebhookUpdate`) | `NotifyUpdate` выходит на первой строке при пустом списке, а список пуст по построению: конфиг собирается литералом, файл конфига не читается |
| metrics (сервис, счётчики в 15 местах, `updateMetrics`, `prometheus/client_golang`, `oklog/run`) | реестр публикует `metricsServer`, который поднимается только при непустом `PrometheusAddress`, а он пуст всегда |
| audit (сервис, 315 вызовов в 19 файлах `api/`, третий параметр `StampModificationMetadata`) | `NewAudit` пишет в `mlog.Discard()`, `Configure` получает два пустых поля конфига |
| compliance (стор, SQL, типы, обёртки `app/`, storetests, три метода `client.go`) | ни одного маршрута; методы клиента шли в `/admin/*`, то есть в выключенный сокет |
| cloud card limits (`sqlstore/cloud.go`, три метода интерфейса, `CardLimitTimestampSystemKey`, `UPDATE_CARD_LIMIT_TIMESTAMP` в ws) | `Block.ShouldBeLimited`/`GetLimited` никто не зовёт, `limited` всегда false; вебсокет-действие не рассылается ниоткуда |
| `GET /statistics` + `app/statistics.go` + `model.BoardsStatistics` + `PermissionGetAnalytics` | живой маршрут без единого вызывающего на странице |
| data retention (`RunDataRetention`, SQL, `EnableDataRetention`, `DataRetentionDays`) | единственный вызывающий — собственный стор-тест; поля конфига не читает никто |
| notification hints (четыре метода, SQL, `model.NotificationHint`) | бэкенд, который их читал, удалён раньше |
| local mode (`EnableLocalMode`, unix-сокет, `localRouter`, `api/admin.go`, `adminRequired`, `admin-scripts/`, conn в `api/context.go`) | флаг выставлен в `false` в единственном месте, где собирается конфиг; на роутере висел один маршрут — сброс пароля через `/var/tmp/focalboard_local.socket` |
| `server/swagger/` (2,1 МБ) | генерируется целью `make swagger` в дереве без Makefile; описывает эндпоинты, которых нет (последнее место, где упоминался insights) |
| `server/app/templates.boardarchive` (6,1 МБ) | embed'ится `server/assets/templates.boardarchive`, это другой файл, и они различаются |
| `ws/plugin_adapter_client.go`, `integrationtests/pluginteststore.go` | последние два файла половины-плагина, на них не ссылается ничто |
| notifylogger | пишет на debug в логгер, который собран на info и не перенастраивается |
| чтение `config.json` (`config.ReadConfigFile`, `spf13/viper` и с ним около 25 пакетов: hcl, toml, ini, afero, fsnotify…) | ни одного вызова: конфиг собирает приложение (см. строку про webhook); сама структура `Configuration` жива |
| `sqlstore.concatenationSelector`, `elementInColumn` и их тесты | звали только собственные тесты (`docs/sql-dialects.md`, группа 0) |

## Слой 2 — мёртвые вызовы страницы и парные маршруты

| Что | Почему было мертво |
| --- | --- |
| `octoClient`: `getUsersList`, `getBlocksWithParent`, `getBlocksWithType`, `patchBlocks`, `getBlocksForBoard`, `createBoard`, `getUserBlockSubscriptions` | ни одного вызова из живого кода; четыре из них вдобавок бьют в маршруты, которых на сервере уже нет (`/teams/{t}/blocks`, `/teams/{t}/boards/{b}`, `POST /teams/{t}/boards`) |
| `octoClient.searchLinkableBoards` + `GET /teams/{teamID}/boards/search/linkable` + `TestPermissionsSearchTeamLinkableBoards` | доски, привязываемые к каналу Mattermost; обработчик и так отвечал «not permitted in standalone mode» |
| канальная половина «Поделиться»: `searchUserChannels`, `getChannel`, `store/channels.ts`, `onLinkBoard` и диалог привязки в `shareBoard.tsx`, унии `IUser \| Channel` | список опций строит `loadShareOptions` из `searchTeamUsers` — положить в него `Channel` было нечему; людская половина диалога цела |
| insights: `getMyTopBoards`, `getTeamTopBoards`, `src/insights/` | вызывающих нет, маршрутов на сервере тоже давно нет |
| скрытые карточки целиком: апселл «Upgrade to Professional or Enterprise» в карточке, `HiddenCardCount` в канбане/таблице/галерее, `cardLimitNotification.tsx` со снузом на 10 дней, `limitCard`/`setLimitTimestamp`/`refreshCards`/`cardHiddenWarning` в сторе карточек, `addOnCardLimitTimestampChange` в `wsclient`, `Block.limited`, `svg/card-skeleton`, 10 ключей в 26 каталогах | `limited` не выставляет никто: `ShouldBeLimited`/`GetLimited` на сервере без вызывающих, вебсокет-действие не рассылается, стор лимитов кормился маршрутом, которого нет |
| `components/globalHeader/` | не импортируется ничем; удалён вместе со скрытыми карточками, потому что был последней ссылкой на снятый `prepareOnboarding` |
| onboarding: `prepareOnboarding`, `POST /teams/{teamID}/onboard`, `app.PrepareOnboardingTour`, `pages/welcome/`, `src/onboardingTour/index.ts`, `TestPermissionsOnboard` | оба вызывающих недостижимы — у `welcomePage` нет маршрута, `globalHeader` не монтируется. **Сам тур жив**: `components/onboardingTour/` — другое, прогресс хранится через `patchUserConfig` |

## Слой 4 — хвосты Mattermost на странице

| Что | Почему было мертво |
| --- | --- |
| плагин-режим в `wsclient.ts`: `MMWebSocketClient`, `initPlugin`, поля `client`/`pluginId`/`pluginVersion`/`clientPrefix`, ветка `open()` на чужом сокете, форки в `sendCommand`/`hasConn`/`authenticate`, `pluginStatusesChangedHandler`; пропсы `manifest`/`webSocketClient` в `withWebSockets` | точки входа в режим нет с тех пор, как `index.html` ведёт в `main.tsx`; решение 3 закрыло режим официально |
| `newVersionBanner` | поднимался только `pluginStatusesChangedHandler`, которого никто не зовёт; у приложения свой апдейтер |
| `components/messages/versionMessage` | показывался при `me().id !== 'single-user'`, что здесь никогда не так, а «Learn more» вело на mattermost.com |
| `setMattermostTheme` | собственный комментарий говорил, что в репозитории её не зовёт никто и она для плагин-хоста |
| `Utils.isDesktopApp`, `getDesktopVersion`, `isDesktop` | нюхают юзер-агент Mattermost-Electron и **врут** в сборке Wails; вызывающих нет |
| `SuiteWindow`, `frontendBaseURL`, `openPricingModal` в `types/index.d.ts` | глобалы, которые ставил только плагин-хост |
| `src/telemetry/` целиком: `TelemetryClient`, 33 вызова `trackEvent`, `setUser`, проп `telemetryTag` в одиннадцати шагах тура, `telemetryName` в `Constants.imports` | `trackEvent` — no-op, пока не позван `setTelemetryHandler`, а звала его только точка входа плагина |
| маршрут `/team/:teamId/new/:channelId`, проп `channelId`, `getBoardPagePath` | вход «создать доску для канала»; не просто мёртв — каждое создание доски из шаблона платило лишним `updateBoard` с пустым `channelId` |

## Слои 3 и 5 — не удаления, а починки

| Что | Что было |
| --- | --- |
| `TestPermissionsGetTeamTemplates` | падал на каждом прогоне против константы `builtInTemplateCount := 13` при 14 шаблонах в архиве. Теперь число спрашивается у стора: тест про права, а не про количество. С этим сьют Go стал зелёным целиком |
| `internal/appschema.OpenEnforcing` | звал `sqlstore.SQLiteDSN`, выбираемый тегом сборки, при драйвере `mattn`, который пакет импортирует безусловно: в сборке без тегов DSN был modernc'шный, ключи выключены, три cascade-теста падали. DSN пишется под свой драйвер, прагма читается обратно. С 2026-09-15 пакет не импортирует драйвер сам: и драйвер, и DSN приходят из `sqlstore`; с 2026-09-16 драйвер один, modernc |
| `BoardGrant.Property` | брался из `cfg.TriggerProperty` — одно имя на все доски, по умолчанию «Статус». Берётся с доски (`columnProperty`), имя из настроек осталось запасным вариантом |

## Что осталось стоять рядом с удалённым

- `POST /users`, `GET /subscriptions/{subscriberID}`, `POST /boards` — маршруты
  живы: их зовёт `server/client/client.go` из интеграционных тестов, и один из
  этих тестов (`TestGetUserListSingleUserWithRealAccounts`) написан уже под этот
  форк, про аккаунты агентов. Со страницы ушли только клиентские методы.
- `SearchBoardsForUserInTeam` — после ухода linkable у него не осталось
  вызывающих в `api/`, но это поиск в пределах одной команды, и он остаётся по
  тому же правилу, что `search` и `getTeamUsers`.
- Таблица `notification_hints` и поля её частоты в конфиге — оставлены: это
  очередь рассылки подписчикам, и в командном режиме уведомления строятся на
  ней заново (`docs/todo.md`). `sharing` в этот пункт попала по ошибке: её стор
  (`sqlstore/sharing.go`), маршрут и тесты живы — это публичный шаринг, который
  оставлен.
- `getFrontendBaseURL` и `getBoardPagePath` не удалены, а схлопнуты: первая
  отличалась от `getBaseURL` только чтением `window.frontendBaseURL`, вторая без
  канального маршрута была тождественной функцией. Вызывающие переписаны на то,
  что осталось.
- Ссылки «Trello / Notion / Todoist» не удалены, а **исправлены**: вели на
  `docs.mattermost.com` с якорями, которые придумало это меню, теперь ведут на
  свой `docs/guide/transfer`. Отправлять человека в документацию чужого продукта
  хуже, чем не отправлять никуда.
- **Подписка на карточку** (`followBlock`/`unfollowBlock` в `mutator` и
  `octoClient`, маршруты `/subscriptions`). Кнопки на экране нет — класс
  `cardFollowBtn` носит кнопка «Attach», — но следить за карточкой имеет смысл
  там, где есть кому рассказать, то есть это командная работа, и по этому же
  правилу остаются логин, шаринг и команды.
- Комментарии `swagger:operation` в обработчиках — они читаются как описание
  маршрута независимо от того, генерирует ли из них кто-нибудь.
