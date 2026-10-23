import { RoundedBox } from '@react-three/drei'
import { useMemo } from 'react'
import * as THREE from 'three'
import type { ProductForm } from '../lib/types'

// Every model is built from primitives and lathe profiles: no files to load.

interface Shape {
  a: string
  b: string
}

function Glossy({ color, roughness = 0.28 }: { color: string; roughness?: number }) {
  return <meshPhysicalMaterial color={color} roughness={roughness} clearcoat={1} clearcoatRoughness={0.15} />
}

/** A two-tone capsule: two hemisphere-capped halves meeting in the middle. */
export function Capsule({ a, b }: Shape) {
  const r = 0.34
  const half = 0.42
  return (
    <group>
      {[a, b].map((color, i) => {
        const dir = i === 0 ? 1 : -1
        return (
          <group key={i} position={[0, (dir * half) / 2, 0]}>
            <mesh castShadow>
              <cylinderGeometry args={[r, r, half, 40, 1, true]} />
              <Glossy color={color} />
            </mesh>
            <mesh position={[0, (dir * half) / 2, 0]} rotation={[dir === 1 ? 0 : Math.PI, 0, 0]} castShadow>
              <sphereGeometry args={[r, 40, 20, 0, Math.PI * 2, 0, Math.PI / 2]} />
              <Glossy color={color} />
            </mesh>
          </group>
        )
      })}
    </group>
  )
}

/** A round tablet with a soft rim and a score line across the top. */
export function Tablet({ a }: Shape) {
  const geo = useMemo(() => {
    const pts: THREE.Vector2[] = []
    const R = 0.62
    const h = 0.16
    const bevel = 0.1
    pts.push(new THREE.Vector2(0, -h - 0.04))
    for (let i = 0; i <= 10; i++) {
      const t = (i / 10) * (Math.PI / 2)
      pts.push(new THREE.Vector2(R - bevel + Math.sin(t) * bevel, -h + bevel - Math.cos(t) * bevel - 0.02))
    }
    for (let i = 0; i <= 10; i++) {
      const t = (i / 10) * (Math.PI / 2)
      pts.push(new THREE.Vector2(R - bevel + Math.cos(t) * bevel, h - bevel + Math.sin(t) * bevel + 0.02))
    }
    pts.push(new THREE.Vector2(0, h + 0.04))
    return new THREE.LatheGeometry(pts, 48)
  }, [])
  return (
    <group rotation={[Math.PI / 2.4, 0, 0]}>
      <mesh geometry={geo} castShadow>
        <meshPhysicalMaterial color={a} roughness={0.55} clearcoat={0.4} />
      </mesh>
      <mesh position={[0, 0.205, 0]}>
        <boxGeometry args={[1.0, 0.03, 0.05]} />
        <meshStandardMaterial color={a} roughness={0.8} />
      </mesh>
    </group>
  )
}

/** A brown glass bottle with a coloured cap and a paper label. */
export function Bottle({ a, b, small = false }: Shape & { small?: boolean }) {
  const geo = useMemo(() => {
    const p = [
      [0, 0], [0.42, 0], [0.48, 0.06], [0.5, 0.2], [0.5, 1.0], [0.46, 1.18], [0.3, 1.32], [0.22, 1.38], [0.22, 1.52], [0, 1.52],
    ].map(([x, y]) => new THREE.Vector2(x, y))
    return new THREE.LatheGeometry(p, 48)
  }, [])
  const s = small ? 0.62 : 1
  return (
    <group scale={s} position={[0, -0.8 * s, 0]}>
      <mesh geometry={geo} castShadow>
        <meshPhysicalMaterial color="#8a4b1c" roughness={0.08} transmission={0.55} thickness={0.6} ior={1.5} clearcoat={1} />
      </mesh>
      <mesh position={[0, 0.6, 0]}>
        <cylinderGeometry args={[0.505, 0.505, 0.5, 48, 1, true]} />
        <meshStandardMaterial color={b} roughness={0.9} side={THREE.DoubleSide} />
      </mesh>
      <mesh position={[0, 0.6, 0]}>
        <cylinderGeometry args={[0.507, 0.507, 0.08, 48, 1, true]} />
        <meshStandardMaterial color={a} roughness={0.6} side={THREE.DoubleSide} />
      </mesh>
      <mesh position={[0, 1.62, 0]} castShadow>
        <cylinderGeometry args={[0.28, 0.28, 0.26, 32]} />
        <Glossy color={a} roughness={0.4} />
      </mesh>
      {small && (
        <mesh position={[0, 1.95, 0]}>
          <coneGeometry args={[0.12, 0.42, 24]} />
          <Glossy color="#ffffff" />
        </mesh>
      )}
    </group>
  )
}

/** An ointment tube: a flattened body crimped at one end. */
export function Tube({ a, b }: Shape) {
  const geo = useMemo(() => {
    const g = new THREE.CylinderGeometry(0.36, 0.36, 1.6, 48, 24)
    const pos = g.attributes.position as THREE.BufferAttribute
    for (let i = 0; i < pos.count; i++) {
      const y = pos.getY(i)
      const t = (y + 0.8) / 1.6 // 0 at the cap end, 1 at the crimp
      const flatten = 1 - t * 0.92
      pos.setZ(i, pos.getZ(i) * flatten)
      pos.setX(i, pos.getX(i) * (1 + t * 0.25))
    }
    g.computeVertexNormals()
    return g
  }, [])
  return (
    <group rotation={[0, 0, 0.25]}>
      <mesh geometry={geo} castShadow>
        <meshPhysicalMaterial color={b} roughness={0.35} metalness={0.15} clearcoat={0.8} />
      </mesh>
      <mesh position={[0, 0.2, 0]} scale={[1.02, 1, 0.7]}>
        <cylinderGeometry args={[0.37, 0.37, 0.12, 48, 1, true]} />
        <meshStandardMaterial color={a} side={THREE.DoubleSide} />
      </mesh>
      <mesh position={[0, -0.95, 0]} castShadow>
        <cylinderGeometry args={[0.18, 0.2, 0.3, 32]} />
        <Glossy color={a} />
      </mesh>
      <mesh position={[0, 0.82, 0]}>
        <boxGeometry args={[0.95, 0.08, 0.06]} />
        <meshStandardMaterial color={b} metalness={0.3} roughness={0.4} />
      </mesh>
    </group>
  )
}

/** A spray can with a nozzle. */
export function Spray({ a, b }: Shape) {
  return (
    <group position={[0, -0.5, 0]}>
      <mesh castShadow>
        <cylinderGeometry args={[0.38, 0.38, 1.3, 48]} />
        <meshPhysicalMaterial color={b} roughness={0.25} metalness={0.4} clearcoat={1} />
      </mesh>
      <mesh position={[0, 0.2, 0]}>
        <cylinderGeometry args={[0.385, 0.385, 0.5, 48, 1, true]} />
        <meshStandardMaterial color={a} side={THREE.DoubleSide} />
      </mesh>
      <mesh position={[0, 0.78, 0]}>
        <sphereGeometry args={[0.38, 48, 16, 0, Math.PI * 2, 0, Math.PI / 2]} />
        <meshPhysicalMaterial color={b} roughness={0.25} metalness={0.4} />
      </mesh>
      <mesh position={[0, 1.18, 0]} castShadow>
        <cylinderGeometry args={[0.13, 0.16, 0.28, 24]} />
        <Glossy color={a} />
      </mesh>
      <mesh position={[0.14, 1.26, 0]} rotation={[0, 0, Math.PI / 2]}>
        <cylinderGeometry args={[0.035, 0.035, 0.16, 12]} />
        <meshStandardMaterial color="#222" />
      </mesh>
    </group>
  )
}

/** A soft square sachet. */
export function Sachet({ a, b }: Shape) {
  return (
    <group rotation={[0.2, 0.3, 0.1]}>
      <RoundedBox args={[1.2, 1.2, 0.12]} radius={0.06} smoothness={4} castShadow>
        <meshPhysicalMaterial color={b} roughness={0.3} metalness={0.5} clearcoat={0.6} />
      </RoundedBox>
      <mesh position={[0, 0, 0.065]}>
        <circleGeometry args={[0.32, 40]} />
        <meshStandardMaterial color={a} />
      </mesh>
    </group>
  )
}

export function FormModel({ form, a, b }: { form: ProductForm } & Shape) {
  switch (form) {
    case 'capsules':
      return <Capsule a={a} b={b} />
    case 'tablets':
      return <Tablet a={a} b={b} />
    case 'syrup':
      return <Bottle a={a} b={b} />
    case 'drops':
      return <Bottle a={a} b={b} small />
    case 'cream':
      return <Tube a={a} b={b} />
    case 'spray':
      return <Spray a={a} b={b} />
    case 'sachets':
      return <Sachet a={a} b={b} />
  }
}
