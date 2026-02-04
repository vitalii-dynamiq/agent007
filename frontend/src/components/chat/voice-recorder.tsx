/**
 * Voice Recorder Component
 * 
 * Elegant inline voice recording with transcription using OpenAI.
 * Similar to ChatGPT's voice input - minimal and unobtrusive.
 */

import { useState, useRef, useEffect, useCallback } from 'react'
import { Mic, Square, Loader2, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { api } from '@/lib/api'

interface VoiceRecorderProps {
  onTranscription: (text: string) => void
  onClose: () => void
  disabled?: boolean
}

export function VoiceRecorder({ onTranscription, onClose, disabled }: VoiceRecorderProps) {
  const [isRecording, setIsRecording] = useState(false)
  const [isTranscribing, setIsTranscribing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [duration, setDuration] = useState(0)
  const [audioLevel, setAudioLevel] = useState(0)
  
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const audioChunksRef = useRef<Blob[]>([])
  const streamRef = useRef<MediaStream | null>(null)
  const analyserRef = useRef<AnalyserNode | null>(null)
  const animationFrameRef = useRef<number | null>(null)
  const timerRef = useRef<NodeJS.Timeout | null>(null)

  // Auto-start recording when component mounts
  useEffect(() => {
    startRecording()
    return () => {
      cleanup()
    }
  }, [])

  const cleanup = () => {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach(track => track.stop())
      streamRef.current = null
    }
    if (timerRef.current) {
      clearInterval(timerRef.current)
      timerRef.current = null
    }
    if (animationFrameRef.current) {
      cancelAnimationFrame(animationFrameRef.current)
      animationFrameRef.current = null
    }
  }

  // Analyze audio levels for visualization
  const analyzeAudio = useCallback(() => {
    const analyser = analyserRef.current
    if (!analyser) return

    const dataArray = new Uint8Array(analyser.frequencyBinCount)
    analyser.getByteFrequencyData(dataArray)

    // Calculate average level
    const average = dataArray.reduce((a, b) => a + b, 0) / dataArray.length
    const normalizedLevel = average / 255
    setAudioLevel(normalizedLevel)

    if (isRecording) {
      animationFrameRef.current = requestAnimationFrame(analyzeAudio)
    }
  }, [isRecording])

  const startRecording = async () => {
    try {
      setError(null)
      
      const stream = await navigator.mediaDevices.getUserMedia({ 
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        }
      })
      streamRef.current = stream

      // Setup audio analyser
      const audioContext = new AudioContext()
      const source = audioContext.createMediaStreamSource(stream)
      const analyser = audioContext.createAnalyser()
      analyser.fftSize = 256
      source.connect(analyser)
      analyserRef.current = analyser

      // Setup media recorder
      const mediaRecorder = new MediaRecorder(stream, {
        mimeType: MediaRecorder.isTypeSupported('audio/webm;codecs=opus') 
          ? 'audio/webm;codecs=opus' 
          : 'audio/webm'
      })
      
      audioChunksRef.current = []
      
      mediaRecorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          audioChunksRef.current.push(event.data)
        }
      }
      
      mediaRecorder.onstop = async () => {
        const audioBlob = new Blob(audioChunksRef.current, { type: 'audio/webm' })
        await transcribeAudio(audioBlob)
      }
      
      mediaRecorderRef.current = mediaRecorder
      mediaRecorder.start(100)
      
      setIsRecording(true)
      setDuration(0)
      
      timerRef.current = setInterval(() => {
        setDuration(d => d + 1)
      }, 1000)
      
      animationFrameRef.current = requestAnimationFrame(analyzeAudio)
      
    } catch (err) {
      console.error('Failed to start recording:', err)
      if (err instanceof Error && err.name === 'NotAllowedError') {
        setError('Microphone access denied')
      } else {
        setError('Failed to access microphone')
      }
    }
  }

  const stopRecording = () => {
    if (mediaRecorderRef.current && isRecording) {
      mediaRecorderRef.current.stop()
    }
    
    cleanup()
    setIsRecording(false)
  }

  const transcribeAudio = async (audioBlob: Blob) => {
    setIsTranscribing(true)
    setError(null)
    
    try {
      const result = await api.transcribeAudio(audioBlob)
      
      if (result.text && result.text.trim()) {
        onTranscription(result.text.trim())
        onClose()
      } else {
        setError('No speech detected')
        setTimeout(onClose, 1500)
      }
    } catch (err) {
      console.error('Transcription failed:', err)
      setError('Transcription failed')
      setTimeout(onClose, 1500)
    } finally {
      setIsTranscribing(false)
    }
  }

  const handleCancel = () => {
    cleanup()
    if (mediaRecorderRef.current && isRecording) {
      mediaRecorderRef.current.stop()
      audioChunksRef.current = [] // Clear audio so it doesn't transcribe
    }
    onClose()
  }

  const formatDuration = (seconds: number) => {
    const mins = Math.floor(seconds / 60)
    const secs = seconds % 60
    return `${mins}:${secs.toString().padStart(2, '0')}`
  }

  // Generate bars for visualization
  const bars = Array.from({ length: 5 }, (_, i) => {
    const baseHeight = 4
    const maxHeight = 24
    const variance = Math.sin((Date.now() / 100) + i) * 0.3 + 0.7
    const height = isRecording 
      ? baseHeight + (audioLevel * maxHeight * variance)
      : baseHeight
    return height
  })

  return (
    <div className="flex items-center gap-3 px-4 py-2 bg-muted/50 rounded-full border animate-in fade-in slide-in-from-bottom-2 duration-200">
      {/* Cancel button */}
      <button
        onClick={handleCancel}
        disabled={isTranscribing}
        className="p-1.5 rounded-full hover:bg-muted transition-colors cursor-pointer disabled:opacity-50"
        title="Cancel"
      >
        <X className="h-4 w-4 text-muted-foreground" />
      </button>

      {/* Waveform visualization */}
      <div className="flex items-center gap-0.5 h-6">
        {bars.map((height, i) => (
          <div
            key={i}
            className={cn(
              "w-1 rounded-full transition-all duration-75",
              isRecording ? "bg-red-500" : "bg-muted-foreground/30"
            )}
            style={{ height: `${height}px` }}
          />
        ))}
      </div>

      {/* Status text */}
      <div className="flex items-center gap-2 min-w-[80px]">
        {isTranscribing ? (
          <span className="text-sm text-muted-foreground flex items-center gap-1.5">
            <Loader2 className="h-3 w-3 animate-spin" />
            Processing
          </span>
        ) : error ? (
          <span className="text-sm text-destructive">{error}</span>
        ) : isRecording ? (
          <span className="text-sm font-mono text-foreground">
            {formatDuration(duration)}
          </span>
        ) : (
          <span className="text-sm text-muted-foreground">Starting...</span>
        )}
      </div>

      {/* Recording indicator / Stop button */}
      {isRecording && !isTranscribing && (
        <button
          onClick={stopRecording}
          className="p-2 rounded-full bg-red-500 hover:bg-red-600 transition-colors cursor-pointer shadow-sm"
          title="Stop recording"
        >
          <Square className="h-3 w-3 fill-white text-white" />
        </button>
      )}

      {/* Pulsing recording dot */}
      {isRecording && !isTranscribing && (
        <span className="relative flex h-2 w-2">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75"></span>
          <span className="relative inline-flex rounded-full h-2 w-2 bg-red-500"></span>
        </span>
      )}
    </div>
  )
}
