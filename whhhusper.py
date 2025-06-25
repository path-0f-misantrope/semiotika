
import os
import traceback
import torch
from flask import Flask, request, jsonify
import whisper



model = whisper.load_model("medium")
app = Flask(__name__)

@app.route("/whhisper", methods=["POST"])
def whisperwork():
    print("запрос получен")
    data = request.get_json()
    file_path = data.get("filename")
    print("Получен путь:", file_path)  # Посмотрим в консоль
    if not file_path or not os.path.isfile(file_path):
        return jsonify({"error": f"Файл не найден: {file_path}"}), 400

    try:
        result = model.transcribe(file_path, language="ru", task="transcribe")
        text = result["text"]
        formatted_text = split_by_words_to_lines(text)
        return jsonify({"transcription": formatted_text})
 
        
    except Exception as e:
        print("Произошла ошибка:")
        traceback.print_exc()  # выведет весь стек ошибки в консоль
        return jsonify({"error": f"Internal Server Error: {str(e)}"}), 500


def split_by_words_to_lines(text, min_words=5, max_words=6):
    words = text.strip().split()
    lines = []
    current_line = []

    for word in words:
        current_line.append(word)
        if len(current_line) >= max_words:
            lines.append(" ".join(current_line))
            current_line = []

    if current_line:
        lines.append(" ".join(current_line))

    return lines







if __name__ == "__main__" :
  app.run(debug=True)