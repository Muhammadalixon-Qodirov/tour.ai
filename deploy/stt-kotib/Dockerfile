# Kotib (o'zbekcha STT, Whisper-medium asosida, Apache-2.0) — serverning o'zida CPU'da.
# 1-bosqich: HuggingFace modelini CTranslate2 int8 formatiga o'girish (faster-whisper uchun ~4 baravar tez, ~0.8 GB RAM).
FROM python:3.11-slim-bookworm AS convert
RUN pip install --no-cache-dir torch==2.4.1 --index-url https://download.pytorch.org/whl/cpu \
 && pip install --no-cache-dir transformers==4.44.2 ctranslate2==4.5.0 huggingface_hub==0.24.6 sentencepiece
ARG MODEL=Kotib/uzbek_stt_v1
RUN python -c "from transformers import AutoTokenizer; AutoTokenizer.from_pretrained('${MODEL}').save_pretrained('/tok')" \
 && ct2-transformers-converter --model ${MODEL} --output_dir /model --quantization int8 \
      --copy_files preprocessor_config.json \
 && cp /tok/tokenizer.json /model/tokenizer.json

# 2-bosqich: ishlash (ffmpeg + faster-whisper)
FROM python:3.11-slim-bookworm
RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg && rm -rf /var/lib/apt/lists/* \
 && pip install --no-cache-dir faster-whisper==1.1.1
COPY --from=convert /model /model
COPY server.py /app/server.py
ENV KOTIB_THREADS=4
EXPOSE 8000
CMD ["python", "-u", "/app/server.py"]
