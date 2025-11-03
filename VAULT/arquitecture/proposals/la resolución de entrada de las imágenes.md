

Entiendo que estás preguntando sobre **la resolución de entrada de las imágenes** que se procesan en el sistema de inferencia de Intel® SceneScape, y si el sistema utiliza modelos específicos para diferentes resoluciones (640 vs 320).<cite/>

## Resolución de Entrada

La resolución **predeterminada es 640x480**. [1](#1-0)  Esta se define como `DEFAULT_RESOLUTION = { w: 640, h: 480 }` en la configuración de cámaras del sistema. [1](#1-0) 

Los archivos de configuración de prueba también confirman que las cámaras están configuradas con resolución 640x480. [2](#1-1) [3](#1-2) [4](#1-3) 

## Modelos y Resoluciones

El sistema **no utiliza modelos ONNX cuantizados específicos para 640 o 320**.<cite/> En su lugar, utiliza **modelos OpenVINO** (formato `.xml` y `.bin`) que se adaptan dinámicamente a la entrada. [5](#1-4) 

### Redimensionamiento Dinámico

El sistema **redimensiona automáticamente** las imágenes de entrada para que coincidan con la forma de entrada del modelo, independientemente de la resolución original de la cámara: [6](#1-5) 

El método `resize()` en `detector.py` se encarga de esto, redimensionando los frames a las dimensiones `(self.w, self.h)` que el modelo requiere. [7](#1-6)  Si `keep_aspect` está habilitado, mantiene la relación de aspecto y agrega padding. [8](#1-7) 

### Forma del Modelo

La forma de entrada del modelo se determina al cargar el modelo: [9](#1-8) 

El sistema lee la forma del modelo desde `self.model.inputs[0].get_partial_shape()` y ajusta el batch size si es necesario. [9](#1-8) 

## Conclusión

El sistema **siempre usa el mismo modelo** configurado, sin importar si la imagen de entrada es 640 o 320.<cite/> El preprocesamiento se encarga de redimensionar la imagen a la resolución que el modelo específico requiera. [10](#1-9)  No hay lógica para seleccionar diferentes modelos basándose en la resolución de entrada.<cite/>

## Notes

Los modelos soportados están listados en `model-config.json` y pueden ser de OpenVINO Model Zoo, Intel Geti™, YOLOv8, o modelos personalizados. [11](#1-10)  Cada modelo tiene su propia resolución de entrada esperada, pero el sistema maneja el redimensionamiento automáticamente en el preprocesamiento.

Wiki pages you might want to explore:
- [Model Management and Integration (open-edge-platform/scenescape)](/wiki/open-edge-platform/scenescape#3.2)
- [OpenVINO Integration (open-edge-platform/scenescape)](/wiki/open-edge-platform/scenescape#3.3)

### Citations

**File:** manager/src/static/js/thing/scenecamera.js (L28-28)
```javascript
const DEFAULT_RESOLUTION = { w: 640, h: 480 };
```

**File:** tests/perf_tests/config/config.json (L14-15)
```json
            "width": 640,
            "height": 480,
```

**File:** tests/perf_tests/config/config.json (L30-31)
```json
            "width": 640,
            "height": 480,
```

**File:** tests/perf_tests/config/config.json (L46-47)
```json
            "width": 640,
            "height": 480,
```

**File:** percebro/src/detector.py (L385-386)
```python
    self.model = self.core.read_model(model=self.model_path,
                            weights=os.path.splitext(self.model_path)[0] + ".bin")
```

**File:** percebro/src/detector.py (L422-425)
```python
    model_shape = list(self.model.inputs[0].get_partial_shape())
    if model_shape[0] != 1:
      model_shape[0] = 1
      self.model.reshape({ self.input_blob: model_shape })
```

**File:** percebro/src/detector.py (L580-594)
```python
  def preprocess(self, input):
    resized = []
    for frame in input.data:
      if np.prod(frame.shape):
        in_frame = self.resize(frame)

        in_frame = self.preprocessColorspace(in_frame)
        if len(frame.shape) > 2:
          in_frame = in_frame.transpose((2, 0, 1))
        in_frame = in_frame.reshape((self.n, self.c, self.h, self.w))
        if self.normalize_input:
          in_frame = np.ascontiguousarray(in_frame).astype(np.float32)
          in_frame /= 255.0
        resized.append(IAData(in_frame, input.id, frame.shape[1::-1]))
    return resized
```

**File:** percebro/src/detector.py (L596-622)
```python
  def resize(self, frame):
    """Resizes frame to maintain the model input width and height. If
    self.keep_aspect is true, it resizes frame without distorting the
    original image (keeping the original aspect ratio) and adds padding
    to maintain the model input width and height."""

    if not self.keep_aspect:
      return cv2.resize(frame, (self.w, self.h))

    width, height = frame.shape[1::-1]
    height_ratio, width_ratio = self.h / height, self.w / width
    resized_width, resized_height = width, height

    # Conditions below are to make sure that both height and width of the
    # resized image is lower than model height and width.
    if height_ratio <= width_ratio:
      resized_width, resized_height = int(width * height_ratio), self.h
    else:
      resized_width, resized_height = self.w, int(height * width_ratio)

    top, left = 0, 0
    bottom, right  = self.h - resized_height, self.w - resized_width
    resized_frame = cv2.resize(frame, (resized_width, resized_height))
    frame_with_padding = cv2.copyMakeBorder(
      resized_frame, top, bottom, left, right, cv2.BORDER_CONSTANT, value=[0, 0, 0]
    )
    return frame_with_padding
```

**File:** percebro/docs/user-guide/How-to-enable-different-models-for-inferencing.md (L5-20)
```markdown
Intel® SceneScape supports various models from OpenVINO™ that are suitable for scene analytics, and for convenience uses a short name to identify each (shown in **bold**):

- **pv0078:** person-vehicle-bike-detection-crossroad-0078
- **pv1016:** person-vehicle-bike-detection-crossroad-1016
- **pv0001:** pedestrian-and-vehicle-detector-adas-0001
- **v0002:** vehicle-detection-adas-0002
- **retail:** person-detection-retail-0013
- **hpe:** human-pose-estimation-0001
- **reid:** person-reidentification-retail-0031
- **pv2000:** person-vehicle-bike-detection-2000
- **pv2001:** person-vehicle-bike-detection-2001
- **pv2002:** person-vehicle-bike-detection-2002
- **v0200:** vehicle-detection-0200
- **v0201:** vehicle-detection-0201
- **v0202:** vehicle-detection-0202

```
