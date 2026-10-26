import {createMemo} from 'solid-js'
import type {Accessor} from 'solid-js'

import octoClient from '../octoClient'
import type {FileInfo} from '../blocks/block'

import {createLoader} from './loader'

// A different file must never display the previous file's URL, including
// while its request is pending or if it fails. The loader drops late replies.
export function createFileLoader(boardId: Accessor<string | undefined>, fileId: Accessor<unknown>) {
    const key = createMemo(() => {
        const file = fileId()
        return {board: boardId() || '', file: typeof file === 'string' ? file : ''}
    }, {equals: (a, b) => a.board === b.board && a.file === b.file})
    const [answer, reload, loading, error] = createLoader(async (wanted) => ({
        ...wanted,
        info: wanted.file ? await octoClient.getFileAsDataUrl(wanted.board, wanted.file) : {},
    }), undefined as {board: string, file: string, info: FileInfo} | undefined, key)
    const info = (): FileInfo => {
        const loaded = answer()
        const wanted = key()
        return loaded?.board === wanted.board && loaded?.file === wanted.file ? loaded.info : {}
    }
    return [info, reload, () => Boolean(key().file) && loading(), error] as const
}
