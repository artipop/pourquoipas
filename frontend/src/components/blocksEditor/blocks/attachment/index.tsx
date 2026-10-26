import {Show, onSettled} from 'solid-js'

import {BlockInputProps, ContentType} from '../types'
import {createFileLoader} from '../../../../hooks/fileLoader'
import {LoadError, LoadingMessage} from '../../../asyncContent'

import './attachment.scss'

type FileInfo = {
    file: string|File
    filename: string
}

const Attachment: ContentType<FileInfo> = {
    name: 'attachment',
    displayName: 'Attachment',
    slashCommand: '/attachment',
    prefix: '',
    runSlashCommand: (): void => {},
    editable: false,
    Display: (props: BlockInputProps<FileInfo>) => {
        const [fileInfo, reload, loading, error] = createFileLoader(() => props.currentBoardId, () => props.value?.file)
        const fileDataUrl = () => fileInfo().url

        return (
            <>
                <Show when={loading()}><LoadingMessage/></Show>
                <Show when={error()}><LoadError onRetry={reload}/></Show>
                <div
                    class='AttachmentView'
                    data-testid='attachment'
                >
                    <a
                        href={fileDataUrl() || '#'}
                        onClick={(e) => e.stopPropagation()}
                        download={props.value.filename}
                    >
                        {'📎'} {props.value.filename}
                    </a>
                </div>
            </>
        )
    },
    Input: (props: BlockInputProps<FileInfo>) => {
        let ref: HTMLInputElement|undefined
        onSettled(() => {
            ref?.click()
        })

        return (
            <input
                ref={ref}
                class='Attachment'
                data-testid='attachment-input'
                type='file'
                onChange={(e) => {
                    const files = e.currentTarget?.files
                    if (files) {
                        for (let i = 0; i < files.length; i++) {
                            const file = files.item(i)
                            if (file) {
                                props.onSave({file, filename: file.name})
                            }
                        }
                    }
                }}
            />
        )
    },
}

Attachment.runSlashCommand = (changeType: (contentType: ContentType<FileInfo>) => void, changeValue: (value: FileInfo) => void): void => {
    changeType(Attachment)
    changeValue({} as any)
}

export default Attachment
