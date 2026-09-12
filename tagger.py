import sys
import transformers


def main(args):

    if len(args) != 3:
        sys.exit('Usage: tagger.py input_dir output_dir')

    humit_tagger = transformers.AutoModel.from_pretrained(
        'Humit-Oslo/humit-tagger-xs', trust_remote_code=True)
    humit_tagger.tag(input_directory=args[1], output_directory=args[2],
                     one_sentence_per_line=True)


if __name__ == '__main__':
    main(sys.argv)
