import {Show} from 'solid-js'
import type {JSX} from '@solidjs/web'

import {IntlShape} from '../../intl'

import {ContentBlock} from '../../blocks/contentBlock'
import {ImageBlock, createImageBlock} from '../../blocks/imageBlock'
import octoClient from '../../octoClient'
import {Utils} from '../../utils'
import ImageIcon from '../../widgets/icons/image'
import {sendFlashMessage} from '../../components/flashMessages'

import {createFileLoader} from '../../hooks/fileLoader'

import {LoadError, LoadingMessage} from '../asyncContent'

import {contentRegistry} from './contentRegistry'
import ArchivedFile from './archivedFile/archivedFile'

type Props = {
    block: ContentBlock
}

const ImageElement = (props: Props): JSX.Element => {
    const [fileInfo, reload, loading, error] = createFileLoader(() => props.block.boardId, () => props.block.fields.fileId)
    const imageDataUrl = () => fileInfo().url

    return (
        <>
            <Show when={loading()}><LoadingMessage/></Show>
            <Show when={error()}><LoadError onRetry={reload}/></Show>
            <Show
                when={!fileInfo().archived}
                fallback={<ArchivedFile fileInfo={fileInfo()}/>}
            >
                <Show when={imageDataUrl()}>
                    <img
                        class='ImageElement'
                        src={imageDataUrl()!}
                        alt={props.block.title}
                    />
                </Show>
            </Show>
        </>
    )
}

contentRegistry.registerContentType({
    type: 'image',
    getDisplayText: (intl: IntlShape) => intl.formatMessage({id: 'ContentBlock.image', defaultMessage: 'image'}),
    getIcon: () => <ImageIcon/>,
    createBlock: async (boardId: string, intl: IntlShape) => {
        return new Promise<ImageBlock>(
            (resolve) => {
                Utils.selectLocalFile(async (file) => {
                    const fileId = await octoClient.uploadFile(boardId, file)

                    if (fileId) {
                        const block = createImageBlock()
                        block.fields.fileId = fileId || ''
                        resolve(block)
                    } else {
                        sendFlashMessage({content: intl.formatMessage({id: 'createImageBlock.failed', defaultMessage: 'Unable to upload the file. File size limit reached.'}), severity: 'normal'})
                    }
                },
                '.jpg,.jpeg,.png,.gif')
            },
        )
    },
    createComponent: (block) => <ImageElement block={block}/>,
})

export default ImageElement
