import { ContactShadows, Environment, Float, Lightformer } from '@react-three/drei'
import { Canvas, useFrame } from '@react-three/fiber'
import { useMemo, useRef, useState } from 'react'
import * as THREE from 'three'
import type { ProductForm } from '../lib/types'
import { FormModel } from './shapes'

const palette = ['#ff7a59', '#2fd3a4', '#a78bfa', '#ffc857', '#5bc0eb', '#ff9ec7', '#ffffff']
const forms: ProductForm[] = ['capsules', 'capsules', 'capsules', 'tablets', 'tablets', 'syrup', 'drops', 'cream', 'spray', 'sachets']

// A deterministic scatter, so the hero looks the same on every visit.
function rng(seed: number) {
  return () => {
    seed = (seed * 16807) % 2147483647
    return (seed - 1) / 2147483646
  }
}

interface Item {
  form: ProductForm
  a: string
  b: string
  pos: [number, number, number]
  rot: [number, number, number]
  scale: number
  speed: number
}

function useItems(count: number): Item[] {
  return useMemo(() => {
    const r = rng(42)
    return Array.from({ length: count }, (_, i) => {
      const angle = r() * Math.PI * 2
      const radius = 1.2 + r() * 3.2
      return {
        form: forms[i % forms.length],
        a: palette[Math.floor(r() * palette.length)],
        b: palette[Math.floor(r() * palette.length)],
        pos: [Math.cos(angle) * radius * 0.9, Math.sin(angle) * radius * 0.8, -r() * 3] as [number, number, number],
        rot: [r() * Math.PI, r() * Math.PI, r() * Math.PI] as [number, number, number],
        scale: 0.55 + r() * 0.55,
        speed: 0.8 + r() * 1.6,
      }
    })
  }, [count])
}

/** One floating item. Clicking gives it a spin that slowly settles. */
function Floater({ item }: { item: Item }) {
  const ref = useRef<THREE.Group>(null)
  const spin = useRef(0)
  const [hover, setHover] = useState(false)
  useFrame((_, dt) => {
    if (!ref.current) return
    spin.current *= 0.96
    ref.current.rotation.y += dt * (0.25 + spin.current)
    const s = item.scale * (hover ? 1.15 : 1)
    ref.current.scale.lerp(new THREE.Vector3(s, s, s), 0.15)
  })
  return (
    <Float speed={item.speed} rotationIntensity={1.2} floatIntensity={1.6}>
      <group
        ref={ref}
        position={item.pos}
        rotation={item.rot}
        scale={item.scale}
        onPointerOver={(e) => {
          e.stopPropagation()
          setHover(true)
          document.body.style.cursor = 'grab'
        }}
        onPointerOut={() => {
          setHover(false)
          document.body.style.cursor = ''
        }}
        onClick={(e) => {
          e.stopPropagation()
          spin.current = 14
        }}
      >
        <FormModel form={item.form} a={item.a} b={item.b} />
      </group>
    </Float>
  )
}

/** The whole cluster leans toward the pointer. */
function Rig({ children }: { children: React.ReactNode }) {
  const ref = useRef<THREE.Group>(null)
  useFrame((state) => {
    if (!ref.current) return
    ref.current.rotation.y = THREE.MathUtils.lerp(ref.current.rotation.y, state.pointer.x * 0.35, 0.05)
    ref.current.rotation.x = THREE.MathUtils.lerp(ref.current.rotation.x, -state.pointer.y * 0.2, 0.05)
  })
  return <group ref={ref}>{children}</group>
}

export default function PillScene({ count = 22, dark = false, offsetX = 0 }: { count?: number; dark?: boolean; offsetX?: number }) {
  const items = useItems(count)
  return (
    <Canvas shadows dpr={[1, 1.8]} camera={{ position: [0, 0, 9], fov: 38 }} gl={{ antialias: true, alpha: true }}>
      <ambientLight intensity={dark ? 0.5 : 0.8} />
      <directionalLight position={[4, 6, 5]} intensity={1.6} castShadow />
      <group position={[offsetX, 0, 0]}>
        <Rig>
          {items.map((it, i) => (
            <Floater key={i} item={it} />
          ))}
        </Rig>
      </group>
      <ContactShadows position={[0, -3.6, 0]} opacity={dark ? 0.5 : 0.28} scale={18} blur={2.6} far={6} />
      <Environment resolution={256}>
        <Lightformer intensity={2.2} position={[0, 5, -6]} scale={[12, 4, 1]} />
        <Lightformer intensity={1.4} position={[-6, 1, 2]} rotation-y={Math.PI / 2} scale={[8, 2, 1]} color="#ffd9c9" />
        <Lightformer intensity={1.4} position={[6, -1, 2]} rotation-y={-Math.PI / 2} scale={[8, 2, 1]} color="#c9fff0" />
      </Environment>
    </Canvas>
  )
}
