---
name: nextflow-examples
version: 0.1.0
description: >
  Real-world Nextflow pipeline examples for IVCAP including genome assembly
  and image analysis, with complete ivcap.yaml configurations.
requires:
  bins: ["ivcap"]
---

# Nextflow Pipeline Examples

This skill provides complete, production-ready examples of Nextflow pipelines for IVCAP.

**See also:**
- Pipeline Basics: `skills://nextflow-pipeline-basics/SKILL.md`
- Production Practices: `skills://nextflow-production-practices/SKILL.md`

---

## Example 1: Genome Assembly Pipeline

```yaml
$schema: urn:ivcap:schema.nextflow.pipeline.1
id: urn:sd-core:nextflow:genome-assembly
name: genome-assembly-pipeline
service-id: urn:ivcap:service:f3a1b2c3-4567-89ab-cdef-1234567890ab
description: |
  De novo genome assembly from long-read sequencing data using Flye assembler.

  ## Purpose
  Assemble complete bacterial or small eukaryotic genomes from PacBio or
  Oxford Nanopore long-read sequencing data. Suitable for genomes 1 Mb - 500 Mb.

  ## Inputs
  - Long-read FASTQ files (PacBio HiFi, PacBio CLR, or Nanopore)
  - Expected genome size estimate
  - Sequencing platform type

  ## Processing Steps
  1. Read quality assessment (NanoPlot)
  2. Read error correction (optional, for CLR/Nanopore)
  3. Assembly graph construction (Flye)
  4. Contig polishing (Racon + Medaka or Pilon)
  5. Assembly quality assessment (QUAST)
  6. Completeness check (BUSCO)

  ## Outputs
  - Final assembly FASTA (scaffolds and contigs)
  - Assembly graph (GFA format)
  - Quality metrics (N50, L50, completeness)
  - QC plots and reports

  ## Dependencies
  - Flye 2.9+
  - NanoPlot 1.40+
  - Racon 1.5+
  - Medaka 1.7+ (for Nanopore)
  - QUAST 5.2+
  - BUSCO 5.4+

  ## References
  Kolmogorov et al. (2019). Assembly of long, error-prone reads using
  repeat graphs. Nature Biotechnology, 37:540-546.

  ## Notes
  - Requires 30-50x coverage for high-quality assembly
  - Memory usage scales with genome size (allow 8-64 GB)
  - PacBio HiFi reads produce best results (fewest errors)

contact:
  name: Genomics Core
  email: genomics@example.org

parameters:
  - name: genome_size
    description: |
      Expected genome size for assembly optimization.
      Used by Flye to estimate coverage and filter contigs.
      Examples: 4.6m (E. coli), 3.2g (human)
      Units: bp (suffix with k/m/g for kilo/mega/giga)
    type: string
    optional: false

  - name: platform
    description: |
      Sequencing platform type (affects error model).
      Valid values:
        - 'pacbio-hifi' (PacBio HiFi/CCS, error rate ~0.1%)
        - 'pacbio-clr' (PacBio CLR, error rate ~10-15%)
        - 'nano-raw' (Nanopore, error rate ~5-10%)
        - 'nano-hq' (Nanopore high-quality, error rate ~3-5%)
    type: string
    optional: false

  - name: min_read_length
    description: |
      Minimum read length to include in assembly.
      Shorter reads are filtered out.
      Recommended: 1000 bp for bacterial genomes, 5000 bp for eukaryotes.
      Units: base pairs (bp)
    type: integer
    optional: true

  - name: polishing_rounds
    description: |
      Number of iterative polishing rounds.
      More rounds improve accuracy but increase runtime.
      Range: 1-4, recommended: 2
      Default: 2
    type: integer
    optional: true

  - name: busco_lineage
    description: |
      BUSCO lineage dataset for completeness assessment.
      Examples: 'bacteria_odb10', 'fungi_odb10', 'metazoa_odb10'
      See https://busco.ezlab.org/ for full list.
      Default: auto-detect from input data
    type: string
    optional: true

samples:
  - name: sample_id
    description: |
      Unique sample identifier (no spaces or special characters).
      Example: isolate_001, strain_XYZ
    type: string

  - name: reads_fastq
    description: |
      Long-read FASTQ file (IVCAP artifact URN or external URL).
      Must be gzip-compressed (.fastq.gz or .fq.gz).
      Can be merged reads from multiple flow cells.
    type: string
    format: uri

  - name: short_reads_r1
    description: |
      Illumina short reads for hybrid polishing (optional).
      Forward read FASTQ file (IVCAP artifact URN or URL).
      Only needed if using Pilon for polishing.
    type: string
    format: uri

  - name: short_reads_r2
    description: |
      Illumina short reads for hybrid polishing (optional).
      Reverse read FASTQ file (IVCAP artifact URN or URL).
    type: string
    format: uri

example:
  $schema: urn:ivcap:schema:genome-assembly-pipeline.request.1
  parameters:
    genome_size: "4.6m"
    platform: "pacbio-hifi"
    min_read_length: 1000
    polishing_rounds: 2
    busco_lineage: "bacteria_odb10"
  samples:
    - sample_id: ecoli_strain_K12
      reads_fastq: "urn:ivcap:artifact:a1b2c3..."
      short_reads_r1: "urn:ivcap:artifact:d4e5f6..."
      short_reads_r2: "urn:ivcap:artifact:g7h8i9..."
```

---

## Example 2: Simple Image Analysis (Single Input)

Even for simple pipelines, define the schema:

```yaml
$schema: urn:ivcap:schema.nextflow.pipeline.1
id: urn:sd-core:nextflow:image-cell-counter
name: image-cell-counter
service-id: urn:ivcap:service:c9d8e7f6-5432-10ab-cdef-9876543210ab
description: |
  Automated cell counting from microscopy images using deep learning.

  ## Purpose
  Count cells in fluorescence microscopy images using a pre-trained
  Mask R-CNN model. Supports common image formats (TIFF, PNG, JPEG).

  ## Inputs
  - Microscopy image (single channel or RGB)
  - Pre-trained model weights (optional, uses default if not provided)

  ## Processing Steps
  1. Image preprocessing (normalization, resizing)
  2. Cell detection and segmentation (Mask R-CNN)
  3. Cell counting and size distribution analysis
  4. Visualization overlay (detected cells highlighted)

  ## Outputs
  - Cell count summary (CSV)
  - Annotated image with detected cells (PNG)
  - Cell size distribution plot (PNG)
  - Detailed cell measurements (area, perimeter, circularity) (CSV)

  ## Dependencies
  - Python 3.9+
  - PyTorch 2.0+
  - Torchvision 0.15+
  - OpenCV 4.7+

  ## References
  He et al. (2017). Mask R-CNN. ICCV 2017.

  ## Notes
  - Optimal for images with 10-1000 cells
  - Model trained on HeLa cells; may need retraining for other cell types
  - Processing time: ~5 seconds per image on GPU, ~30 seconds on CPU

contact:
  name: Imaging Core
  email: imaging@example.org

parameters:
  - name: confidence_threshold
    description: |
      Minimum confidence score for cell detection (0.0-1.0).
      Higher values reduce false positives but may miss faint cells.
      Recommended: 0.5-0.8
      Default: 0.7
    type: number
    optional: true

  - name: min_cell_area
    description: |
      Minimum cell area in pixels to count as valid cell.
      Filters out noise and debris.
      Default: 100 pixels
    type: integer
    optional: true

  - name: model_weights
    description: |
      Custom model weights file (IVCAP artifact URN or URL).
      If not provided, uses default pre-trained weights.
      Format: PyTorch .pth file
    type: string
    format: uri
    optional: true

# Even though this processes one image at a time, define samples schema
# This enables batch processing and provenance tracking
samples:
  - name: image_id
    description: |
      Unique identifier for this image.
      Example: plate1_well_A1, experiment_001
    type: string

  - name: image_file
    description: |
      Microscopy image file (IVCAP artifact URN or external URL).
      Supported formats: .tiff, .tif, .png, .jpg, .jpeg
      Can be grayscale or RGB.
    type: string
    format: uri

  - name: pixel_size_um
    description: |
      Physical pixel size in micrometers (for size measurements).
      Example: 0.65 for 40x objective with typical camera.
      Units: μm/pixel
    type: number

example:
  $schema: urn:ivcap:schema:image-cell-counter.request.1
  parameters:
    confidence_threshold: 0.7
    min_cell_area: 100
  samples:
    - image_id: plate1_well_A1
      image_file: "urn:ivcap:artifact:img123..."
      pixel_size_um: 0.65
    - image_id: plate1_well_A2
      image_file: "urn:ivcap:artifact:img456..."
      pixel_size_um: 0.65
    - image_id: plate1_well_B1
      image_file: "urn:ivcap:artifact:img789..."
      pixel_size_um: 0.65
```

**Notice:** Even though this pipeline processes one image per sample, defining
the samples schema enables:
- Batch processing (multiple images in one job)
- Provenance tracking (which images were analyzed)
- Reusability (easy to run on new datasets)

---

## Validation Checklist

Before deploying your pipeline, verify:

### Documentation Quality

- [ ] Pipeline description is multi-line and comprehensive
- [ ] Description includes: Purpose, Inputs, Steps, Outputs, Dependencies, References
- [ ] Every parameter has a detailed description
- [ ] Parameter descriptions include units, ranges, and defaults
- [ ] Every sample field has a clear description
- [ ] Sample field descriptions explain format and validation requirements

### Schema Completeness

- [ ] Samples schema is defined (even for single-file pipelines)
- [ ] All input data files are referenced through sample fields
- [ ] Sample fields use `format: uri` for artifact references
- [ ] Parameter types are appropriate (`string`, `integer`, `number`, `boolean`)
- [ ] Optional parameters are marked with `optional: true`

### Provenance Readiness

- [ ] All data inputs come through samples (not hard-coded paths)
- [ ] Artifact references use URN format or URLs
- [ ] No direct file paths in `params` defaults
- [ ] Pipeline can process different datasets without code changes

### Example Request

- [ ] Example request is provided in `ivcap.yaml`
- [ ] Example demonstrates typical use case
- [ ] Example shows both parameters and samples
- [ ] Example uses realistic URNs or URLs

### Testing

- [ ] Test with actual IVCAP artifact URNs
- [ ] Verify provenance is recorded correctly
- [ ] Test with different sample counts (1, 2, many)
- [ ] Validate error messages for invalid inputs

---

## Common Mistakes to Avoid

### ❌ Mistake 1: No Samples Schema

```yaml
# Wrong: Only parameters, no samples
parameters:
  - name: input_file
    description: Input file path
    type: string
```

**Why it's wrong:**
- No provenance tracking
- Not reusable
- No type safety

**Fix:**
```yaml
# Correct: Use samples for data inputs
parameters:
  - name: analysis_mode
    description: Analysis mode (fast or thorough)
    type: string

samples:
  - name: input_file
    description: Input data file (IVCAP artifact URN)
    type: string
    format: uri
```

### ❌ Mistake 2: Vague Descriptions

```yaml
# Wrong: Vague description
description: This pipeline processes data.
```

**Fix:**
```yaml
# Correct: Detailed description
description: |
  RNA-seq differential expression analysis using DESeq2.

  ## Purpose
  Identify differentially expressed genes between experimental conditions...
  [continue with full details]
```

### ❌ Mistake 3: Missing Units/Ranges

```yaml
# Wrong: No units or ranges
parameters:
  - name: threshold
    description: Quality threshold
    type: number
```

**Fix:**
```yaml
# Correct: Include units and ranges
parameters:
  - name: quality_threshold
    description: |
      Phred quality score threshold (0-40).
      Recommended: 20 (Q20) or 30 (Q30).
      Default: 20
    type: integer
    optional: true
```

### ❌ Mistake 4: Hard-Coded Data

```groovy
// Wrong: Hard-coded paths in main.nf
params.reference = "/data/hg38.fa"
params.samples = "/data/samples.csv"
```

**Fix:**
```yaml
# ivcap.yaml - Accept data as samples
samples:
  - name: reference_genome
    description: Reference genome FASTA (IVCAP artifact URN)
    type: string
    format: uri
```

```groovy
// main.nf - Extract from IVCAP request
def refGenome = jobParams?.samples[0]?.reference_genome
def refFile = file(refGenome)
```

---

## Summary

Building production-ready Nextflow pipelines for IVCAP requires:

1. **Always define samples schema** - Even for simple/specific use cases
2. **Write comprehensive descriptions** - Pipeline, parameters, and sample fields
3. **Include units and ranges** - Make parameters self-documenting
4. **Use URN references** - Enable provenance tracking
5. **Think reusability** - Design for future datasets, not just current data

**Remember:** Time spent on good documentation and schema design pays off through:
- Better provenance tracking
- Easier reuse by others (and future you)
- Fewer runtime errors
- Better integration with IVCAP ecosystem

Your pipeline is not just code - it's a reusable scientific tool. Build it accordingly.

---

## Next Steps

- **Production Practices**: `skills://nextflow-production-practices/SKILL.md`
- **Pipeline Basics**: `skills://nextflow-pipeline-basics/SKILL.md`
- **Debugging**: `skills://nextflow-debugging/SKILL.md`
