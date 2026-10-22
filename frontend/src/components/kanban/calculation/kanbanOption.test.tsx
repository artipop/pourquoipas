import {render} from '@solidjs/testing-library'

import {TestBlockFactory} from '../../../test/testBlockFactory'
import {wrapIntl} from '../../../testUtils'

import {Option} from './kanbanOption'

describe('components/kanban/calculations/Option', () => {
    const board = TestBlockFactory.createBoard()

    test('base case', () => {
        const component = () => wrapIntl(() =>
            <Option
                data={{
                    label: 'Count Unique Values',
                    displayName: 'Unique',
                    value: 'countUniqueValue',
                    cardProperties: board.cardProperties,
                    onChange: () => {},
                    activeValue: 'count',
                    activeProperty: board.cardProperties[1],
                }}
            />,
        )

        const {container} = render(component)
        expect(container).toMatchSnapshot()
    })

    // The menu is read in the page's language: its rows used to print the
    // option's English label straight from the table of options.
    test('names the calculation in the language of the page', () => {
        const {getByText} = render(() => wrapIntl(() =>
            <Option
                data={{
                    label: 'Count Unique Values',
                    displayName: 'Unique',
                    value: 'countUniqueValue',
                    cardProperties: board.cardProperties,
                    onChange: () => {},
                    activeValue: 'count',
                    activeProperty: board.cardProperties[1],
                }}
            />,
        ))
        expect(getByText('Count unique values')).toBeInTheDocument()
    })
})
