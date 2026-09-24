import {Board} from '../../blocks/board'
import mutator from '../../mutator'
import {IntlShape} from '../../intl'

import {boardAutomationProperties, readBoardSetup} from './automation'

// Turning a board somebody has built into a template other boards can start
// from. The board part of it is Focalboard's own duplicate-as-template; what
// this adds is the two things the copy would otherwise lose — the board's own
// name, and the setup questions it declares.
//
// It used to add a third: what each column does, read back out of the machine's
// registry. There is no registry any more — a column is an option of the
// board's own select, it travels with the copy like every other property, and
// what happens in it is not configured at all.

export function isSaveAsTemplateAvailable(): boolean {
    return true
}

// saveBoardAsTemplate copies the board as a template and gives the copy the
// automation the original runs. It returns the template board, which is what
// the editor then opens.
export async function saveBoardAsTemplate(board: Board, intl: IntlShape): Promise<Board> {
    const copied = await mutator.duplicateBoard(
        board.id,
        intl.formatMessage({id: 'Mutator.new-template-from-board', defaultMessage: 'new template from board'}),
        true,
    )
    const template = copied.boards[0]
    if (!template) {
        throw new Error(intl.formatMessage({id: 'SaveAsTemplate.failed', defaultMessage: 'The board could not be copied.'}))
    }

    // The setup steps travel with it if the board declared any — a board made
    // from a template carries them, and this is that board going back.
    //
    // The name travels too. Focalboard's duplicate-as-template calls the copy
    // "New board template", which is right for a template made from nothing and
    // wrong for this: what was saved is a board somebody built and named, and
    // the dialog that opens next is where the name is changed if it needs to be.
    const updated: Board = {
        ...template,
        title: board.title || template.title,
        properties: boardAutomationProperties(template, readBoardSetup(board)),
    }
    await mutator.updateBoard(updated, template, 'template setup')
    return updated
}
