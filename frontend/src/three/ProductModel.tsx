import { ContactShadows, Environment, Float, Lightformer, PresentationControls } from '@react-three/drei'
import { Canvas } from '@react-three/fiber'
import type { ProductForm } from '../lib/types'
import { FormModel } from './shapes'

/** One product's form, large, draggable, gently bobbing. */
export default function ProductModel({ form, a, b }: { form: ProductForm; a: string; b: string }) {
  return (
    <Canvas shadows dpr={[1, 2]} camera={{ position: [0, 0.4, 4.4], fov: 35 }} gl={{ alpha: true }}>
      <ambientLight intensity={0.7} />
      <directionalLight position={[3, 5, 4]} intensity={1.8} castShadow />
      <PresentationControls global snap rotation={[0.1, -0.5, 0]} polar={[-0.4, 0.4]} azimuth={[-Infinity, Infinity]}>
        <Float speed={2} rotationIntensity={0.4} floatIntensity={0.8}>
          <group scale={1.05} rotation={[0, 0, form === 'capsules' ? -0.75 : 0]}>
            <FormModel form={form} a={a} b={b} />
          </group>
        </Float>
      </PresentationControls>
      <ContactShadows position={[0, -1.5, 0]} opacity={0.35} scale={6} blur={2.4} far={3} />
      <Environment resolution={256}>
        <Lightformer intensity={2.4} position={[0, 4, -5]} scale={[10, 3, 1]} />
        <Lightformer intensity={1.2} position={[-5, 0, 2]} rotation-y={Math.PI / 2} scale={[6, 2, 1]} color="#ffe2d6" />
        <Lightformer intensity={1.2} position={[5, 0, 2]} rotation-y={-Math.PI / 2} scale={[6, 2, 1]} color="#d6fff3" />
      </Environment>
    </Canvas>
  )
}
