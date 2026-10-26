import {Show, onSettled} from 'solid-js'

import {BlockInputProps, ContentType} from '../types'
import {createFileLoader} from '../../../../hooks/fileLoader'
import {LoadError, LoadingMessage} from '../../../asyncContent'

import './video.scss'

type FileInfo = {
    file: string|File
    filename: string
    width?: number
    align?: 'left'|'center'|'right'
}

const Video: ContentType<FileInfo> = {
    name: 'video',
    displayName: 'Video',
    slashCommand: '/video',
    prefix: '',
    runSlashCommand: (): void => {},
    editable: false,
    Display: (props: BlockInputProps<FileInfo>) => {
        const [fileInfo, reload, loading, error] = createFileLoader(() => props.currentBoardId, () => props.value?.file)
        const videoDataUrl = () => fileInfo().url

        return (
            <>
                <Show when={loading()}><LoadingMessage/></Show>
                <Show when={error()}><LoadError onRetry={reload}/></Show>
                <Show when={videoDataUrl()}>
                    <video
                        width='320'
                        height='240'
                        controls={true}
                        class='VideoView'
                        data-testid='video'
                    >
                        <source src={videoDataUrl()!}/>
                    </video>
                </Show>
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
                class='Video'
                data-testid='video-input'
                type='file'
                accept='video/*'
                onChange={(e) => {
                    const file = (e.currentTarget?.files || [])[0]
                    props.onSave({file, filename: file.name})
                }}
            />
        )
    },
}

Video.runSlashCommand = (changeType: (contentType: ContentType<FileInfo>) => void, changeValue: (value: FileInfo) => void): void => {
    changeType(Video)
    changeValue({} as any)
}

export default Video
