import {defineMessages} from '@formatjs/intl'

import {IntlShape} from './intl'

import {Constants} from './constants'

// What a colour is called on screen. The ids are the option colour ids the
// board stores (Constants.menuColors keys); the words a person reads come
// from the catalogue — three menus offered «Gray» and «Brown» on a Russian
// board because the constant's English value went straight into the row.
// defineMessages, not a bare object: the extractor reads only what is handed
// to a call it knows, and a bare object left the ten names out of en.json.
const NAMES: {[key: string]: {id: string, defaultMessage: string}} = defineMessages({
    propColorDefault: {id: 'MenuColors.default', defaultMessage: 'Default'},
    propColorGray: {id: 'MenuColors.gray', defaultMessage: 'Gray'},
    propColorBrown: {id: 'MenuColors.brown', defaultMessage: 'Brown'},
    propColorOrange: {id: 'MenuColors.orange', defaultMessage: 'Orange'},
    propColorYellow: {id: 'MenuColors.yellow', defaultMessage: 'Yellow'},
    propColorGreen: {id: 'MenuColors.green', defaultMessage: 'Green'},
    propColorBlue: {id: 'MenuColors.blue', defaultMessage: 'Blue'},
    propColorPurple: {id: 'MenuColors.purple', defaultMessage: 'Purple'},
    propColorPink: {id: 'MenuColors.pink', defaultMessage: 'Pink'},
    propColorRed: {id: 'MenuColors.red', defaultMessage: 'Red'},
})

export function menuColorLabel(intl: IntlShape, key: string): string {
    const message = NAMES[key]
    return message ? intl.formatMessage(message) : Constants.menuColors[key] || key
}
