from pathlib import Path
import math

from PIL import Image, ImageDraw, ImageFont
from reportlab.pdfgen import canvas
from reportlab.lib.utils import ImageReader
from pypdf import PdfReader


OUT = Path(__file__).resolve().parent
W, H = 1920, 1080
BG = '#F7F8FA'
INK = '#172C3F'
MUTED = '#506475'
LINE = '#687C8D'
BLUE = '#1D5D8F'
LIGHT_BLUE = '#E4F0FB'
COLORS = {
    'actor': ('#F5DD95', '#705615'),
    'command': ('#CDE6FA', '#174E78'),
    'aggregate': ('#FFF0A8', '#796014'),
    'event': ('#FFD3A5', '#8D4312'),
    'policy': ('#E7DDF3', '#60427E'),
    'read': ('#D6EEDB', '#315E3C'),
}
FONT = Path('C:/Windows/Fonts/segoeui.ttf')
BOLD = Path('C:/Windows/Fonts/segoeuib.ttf')
assert FONT.exists() and BOLD.exists()


def font(size, bold=False):
    return ImageFont.truetype(str(BOLD if bold else FONT), size)


def wrap(text, size, width, bold=False):
    f = font(size, bold)
    lines = []
    for paragraph in text.split('\n'):
        line = ''
        for word in paragraph.split():
            candidate = (line + ' ' + word).strip()
            if f.getlength(candidate) <= width:
                line = candidate
            else:
                if line:
                    lines.append(line)
                if f.getlength(word) > width:
                    raise ValueError(f'Palavra larga demais: {word}')
                line = word
        lines.append(line)
    return lines


class Diagram:
    def __init__(self, label, title, subtitle, page):
        self.im = Image.new('RGB', (W, H), BG)
        self.d = ImageDraw.Draw(self.im)
        self.text((72, 30, 1776, 36), label.upper(), 25, BLUE, True)
        self.text((72, 78, 1776, 76), title, 54, INK, True)
        self.text((72, 154, 1776, 56), subtitle, 28, MUTED)
        self.d.line((72, 1024, 1848, 1024), fill='#CDD6DE', width=2)
        self.text((72, 1036, 1500, 34), 'FIAP SOAT | Fase 1 | Grupo 1 | Lucas Velôso | RM 377934', 23, MUTED)
        self.text((1748, 1036, 100, 34), page, 23, MUTED, align='right')

    def text(self, bounds, text, size=28, color=INK, bold=False, align='left', valign='top'):
        x, y, width, height = bounds
        lines = wrap(text, size, width, bold)
        line_h = round(size * 1.28)
        total_h = len(lines) * line_h
        if total_h > height:
            raise ValueError(f'Texto excede altura ({total_h}>{height}): {text}')
        if valign == 'center':
            y += (height - total_h) / 2
        for line in lines:
            line_w = font(size, bold).getlength(line)
            dx = (width - line_w) / 2 if align == 'center' else width - line_w if align == 'right' else 0
            self.d.text((x + dx, y), line, fill=color, font=font(size, bold))
            y += line_h

    def panel(self, box, fill='white', outline='#D3DCE4', radius=16, width=2):
        self.d.rounded_rectangle(box, radius=radius, fill=fill, outline=outline, width=width)

    def arrow(self, pts, color=LINE, width=4, head=14, dash=False):
        if dash:
            for a, b in zip(pts, pts[1:]):
                length = math.dist(a, b)
                for start in range(0, int(length), 17):
                    end = min(start + 9, length)
                    self.d.line((a[0] + (b[0]-a[0])*start/length, a[1] + (b[1]-a[1])*start/length,
                                 a[0] + (b[0]-a[0])*end/length, a[1] + (b[1]-a[1])*end/length), fill=color, width=width)
        else:
            self.d.line(pts, fill=color, width=width, joint='curve')
        if head:
            a, b = pts[-2:]
            angle = math.atan2(b[1]-a[1], b[0]-a[0])
            vertices = [b,
                        (b[0]-head*math.cos(angle-.5), b[1]-head*math.sin(angle-.5)),
                        (b[0]-head*math.cos(angle+.5), b[1]-head*math.sin(angle+.5))]
            self.d.polygon(vertices, fill=color)

    def card(self, box, title, kind, body, fill=BLUE, size=32):
        x1, y1, x2, y2 = box
        self.panel(box, fill, fill)
        self.text((x1+24, y1+20, x2-x1-48, 46), kind.upper(), 23, '#D6E8F7', True, align='center')
        title_lines = wrap(title, size, x2-x1-48, True)
        title_h = len(title_lines) * round(size*1.28)
        self.text((x1+24, y1+70, x2-x1-48, title_h), title, size, 'white', True, align='center')
        self.text((x1+24, y1+86+title_h, x2-x1-48, y2-y1-110-title_h), body, 27, 'white', align='center')

    def note(self, box, title, body):
        x1, y1, x2, y2 = box
        self.panel(box, '#EAF0F5', '#EAF0F5')
        self.text((x1+24, y1+16, x2-x1-48, 42), title, 28, INK, True)
        self.text((x1+24, y1+62, x2-x1-48, y2-y1-70), body, 27, MUTED)

    def save(self, stem):
        self.im.save(OUT / (stem + '.png'), optimize=True)
        return OUT / (stem + '.png')


def c4_context():
    p = Diagram('C4 / Nível 1', 'Contexto da oficina', 'Quem usa o sistema e para qual finalidade.', '1 / 3')
    p.card((82, 332, 442, 630), 'Funcionário', 'Pessoa', 'Gerencia cadastros,\norçamentos, estoque\ne execução.', '#38556E')
    p.card((700, 302, 1220, 678), 'Sistema da oficina', 'Sistema de software', 'API para atendimento e\nexecução de serviços.\n\nPreserva o histórico\nda ordem de serviço.', BLUE, 39)
    p.card((1478, 332, 1838, 630), 'Cliente', 'Pessoa', 'Consulta sua OS\ne aprova ou rejeita\no orçamento.', '#38556E')
    p.text((462, 363, 218, 112), 'Cadastros,\norçamento\ne execução', 27, MUTED, align='center')
    p.arrow([(442, 502), (700, 502)])
    p.text((1240, 363, 218, 112), 'Andamento\ne decisão\ndo orçamento', 27, MUTED, align='center')
    p.arrow([(1478, 502), (1220, 502)])
    p.text((457, 528, 230, 70), 'Acesso administrativo', 25, BLUE, True, align='center')
    p.text((1238, 528, 225, 70), 'Acesso limitado à OS', 25, BLUE, True, align='center')
    p.note((82, 790, 1838, 946), 'Escopo da fase 1', 'Um backend monolítico. Funcionário e cliente interagem por API; o Swagger permite demonstrar as operações.')
    return p.save('c4-n1-contexto')


def c4_containers():
    p = Diagram('C4 / Nível 2', 'Contêineres da solução', 'A API serve o contrato e a documentação interativa no mesmo processo Go.', '2 / 3')
    p.panel((578, 232, 1850, 746), '#F0F4F7', '#A8BAC9')
    p.text((608, 248, 1160, 40), 'SISTEMA DA OFICINA', 25, BLUE, True)
    p.card((78, 335, 408, 644), 'Consumidor HTTP', 'Ferramenta de acesso', 'Funcionário ou cliente.\nPode usar Swagger\nno navegador.', '#38556E', 32)
    p.card((630, 324, 1172, 680), 'API da oficina', 'Contêiner / Go + Gin', 'Operações REST /api/v1\nSwagger /docs\nContrato /openapi.yaml\nJWT e token da OS', BLUE, 38)
    p.card((1452, 324, 1810, 680), 'PostgreSQL 18', 'Contêiner / Banco', 'Cadastros e OS\nem JSONB.\nMovimentos, restrições\ne transações.', '#276C85', 32)
    p.text((424, 362, 190, 100), 'HTTP / JSON\nBearer', 27, MUTED, align='center')
    p.arrow([(408, 495), (630, 495)])
    p.text((1186, 360, 252, 108), 'SQL via pgx\nTransações', 27, MUTED, align='center')
    p.arrow([(1172, 495), (1452, 495)])
    p.panel((78, 822, 608, 974), '#E7E9ED', '#BCC4CC')
    p.text((98, 837, 490, 42), 'jsDelivr / sistema externo', 28, INK, True, align='center')
    p.text((98, 886, 490, 70), 'Assets Swagger UI 5.33.1\nCSS e JavaScript', 26, MUTED, align='center')
    p.arrow([(244, 644), (244, 822)], dash=True)
    p.text((274, 704, 285, 75), 'HTTPS\nSomente os assets', 25, MUTED)
    p.note((664, 822, 1810, 974), 'Execução local', 'API e banco no próprio computador. A internet é necessária para carregar os assets do Swagger.')
    return p.save('c4-n2-containers')


def component(p, box, title, file, body, fill=LIGHT_BLUE):
    x1, y1, x2, y2 = box
    p.panel(box, fill, '#91AFC6')
    p.text((x1+20, y1+16, x2-x1-40, 78), title, 29, BLUE, True, align='center')
    p.text((x1+18, y1+97, x2-x1-36, 66), file, 23, MUTED, align='center')
    if body:
        p.text((x1+18, y1+166, x2-x1-36, y2-y1-176), body, 25, INK, align='center')


def c4_components():
    p = Diagram('C4 / Nível 3', 'Início do primeiro serviço', 'Componentes do backend que autorizam, validam e persistem uma única operação.', '3 / 3')
    p.text((78, 222, 1770, 42), 'Funcionário → POST /api/v1/admin/service-orders/{os}/services/{item}/start + expectedVersion', 27, MUTED)
    p.panel((64, 279, 1856, 831), '#EFF4F8', '#A8BAC9')
    p.text((86, 289, 1700, 35), 'API DA OFICINA / Go + Gin', 23, BLUE, True)
    boxes = [(90, 345, 454, 563), (530, 345, 894, 563), (970, 345, 1354, 563), (1430, 345, 1830, 563)]
    component(p, boxes[0], 'Entrada e autorização', 'server.New / auth.Admin', 'Valida o JWT administrativo.')
    component(p, boxes[1], 'Handler de OS', 'workorders/http.go\ncommand', 'Valida IDs e expectedVersion.')
    component(p, boxes[2], 'Caso de uso', 'workorders.Service.Execute', 'Coordena a transação pgx.')
    component(p, boxes[3], 'Regras de domínio', 'domain.Order\nCheckVersion / StartService', 'Versão, aprovação e estado.')
    for left, right in zip(boxes, boxes[1:]):
        p.arrow([(left[2], 449), (right[0], 449)])
    lower = [(285, 633, 705, 798), (810, 633, 1230, 798), (1335, 633, 1755, 798)]
    for box, title, body in [
        (lower[0], 'Persistência da OS', 'workorders.Load / Save\nSELECT FOR UPDATE'),
        (lower[1], 'Baixa dos itens', 'consume / platform.Get e Save\ndomain.AdjustStock'),
        (lower[2], 'Registro dos movimentos', 'registry.InsertMovement\nINSERT vinculado à OS'),
    ]:
        x1,y1,x2,y2=box
        p.panel(box, 'white', '#91AFC6')
        p.text((x1+15,y1+17,x2-x1-30,48), title, 27, BLUE, True, align='center')
        p.text((x1+15,y1+76,x2-x1-30,80), body, 24, INK, align='center')
    p.arrow([(1130,563),(1130,597),(495,597),(495,633)], head=13)
    p.arrow([(1160,563),(1160,609),(1020,609),(1020,633)], head=13)
    p.arrow([(1190,563),(1190,597),(1545,597),(1545,633)], head=13)
    for x in [495,1020,1545]:
        p.arrow([(x,798),(x,867)], head=15)
    p.panel((285,867,1755,949), '#276C85', '#276C85')
    p.text((305,883,1430,48), 'PostgreSQL 18 | OS + estoque + movimentos | mesma transação', 30, 'white', True, align='center')
    p.text((76,973,1760,40), 'Primeiro início: baixa todas as peças em ordem estável. Falha: rollback. Sucesso: salva a OS e confirma.', 25, INK, align='center')
    return p.save('c4-n3-primeiro-servico')


def sticky(p, x, y, width, height, kind, label, text, size=27):
    fill, ink = COLORS[kind]
    p.panel((x,y,x+width,y+height), fill, fill, radius=9)
    p.text((x+15,y+9,width-30,31), label.upper(), 20, ink, True)
    p.text((x+15,y+45,width-30,height-50), text, size, INK, True if kind in ['event','aggregate'] else False)


def es(title, subtitle, stem, columns, page, note):
    p=Diagram('Event Storming', title, subtitle, page)
    count=len(columns)
    gap=24
    width=(1776-gap*(count-1))//count
    for index,col in enumerate(columns):
        x=72+index*(width+gap)
        sticky(p,x,235,width,82,'actor','Ator',col['actor'],25)
        p.arrow([(x+width/2,317),(x+width/2,338)],width=3,head=10)
        sticky(p,x,338,width,124,'read' if col.get('query') else 'command','Consulta' if col.get('query') else 'Comando',col['command'],27)
        p.arrow([(x+width/2,462),(x+width/2,483)],width=3,head=10)
        sticky(p,x,483,width,82,'aggregate','Agregado',col['aggregate'],25)
        p.arrow([(x+width/2,565),(x+width/2,586)],width=3,head=10)
        sticky(p,x,586,width,132,'read' if col.get('query') else 'event','Leitura' if col.get('query') else 'Evento',col['event'],27)
        if index<count-1:
            p.arrow([(x+width,650),(x+width+gap,650)],width=3,head=9)
        p.arrow([(x+width/2,718),(x+width/2,742)],width=2,head=0,dash=True)
        sticky(p,x,742,width,162,'policy','Política / regra',col['policy'],24)
    p.text((72,925,1776,44),note,25,INK)
    legend=[('actor','Ator'),('command','Comando'),('aggregate','Agregado'),('event','Evento'),('policy','Política'),('read','Leitura')]
    x=72
    for kind,label in legend:
        fill,ink=COLORS[kind]
        p.panel((x,981,x+25,1006),fill,fill,radius=3)
        p.text((x+35,977,200,38),label,24,MUTED)
        x+=274
    return p.save(stem)


def event_diagrams():
    creation=[
        dict(actor='Funcionário',command='Cadastrar cliente\ne veículo',aggregate='Cliente / Veículo',event='Cliente e veículo\ncadastrados',policy='Documento e placa únicos. Veículo vinculado ao cliente.'),
        dict(actor='Funcionário',command='Abrir OS',aggregate='Ordem de serviço',event='OS recebida',policy='Cadastros ativos e vinculados. Preservar dados da abertura.'),
        dict(actor='Funcionário',command='Iniciar diagnóstico',aggregate='Ordem de serviço',event='Diagnóstico\niniciado',policy='OS recebida passa a em diagnóstico.'),
        dict(actor='Funcionário',command='Definir serviços\ne peças',aggregate='Ordem de serviço',event='Itens da OS\ndefinidos',policy='Só recebida / diagnóstico. Itens únicos; quantidade positiva.'),
        dict(actor='Funcionário',command='Publicar orçamento',aggregate='Ordem de serviço',event='Orçamento\npublicado',policy='Exige serviço. Congela preços e total; aguarda aprovação.'),
    ]
    tracking=[
        dict(actor='Cliente autorizado',command='Consultar OS',aggregate='Ordem de serviço',event='Visão restrita\nda OS',policy='Token limita acesso. A consulta não muda o estado.',query=True),
        dict(actor='Cliente autorizado',command='Aprovar ou rejeitar\norçamento',aggregate='Ordem de serviço',event='Orçamento\naprovado / rejeitado',policy='Só vigente e pendente. Rejeitar: diagnóstico. Aprovar: aguarda início.'),
        dict(actor='Funcionário',command='Iniciar serviço',aggregate='OS / Estoque',event='Serviço iniciado;\nexecução iniciada',policy='Exige aprovação. Primeiro início baixa peças com transação.'),
        dict(actor='Funcionário',command='Concluir serviços;\nfinalizar OS',aggregate='Ordem de serviço',event='Serviços concluídos;\nOS finalizada',policy='Concluir só em execução. Finalizar exige todos os serviços concluídos.'),
        dict(actor='Funcionário',command='Registrar entrega',aggregate='Ordem de serviço',event='OS entregue',policy='Exige OS finalizada. Preservar histórico e horários.'),
    ]
    stock=[
        dict(actor='Funcionário',command='Cadastrar item',aggregate='Item de estoque',event='Item cadastrado;\nabertura registrada*',policy='SKU único; saldo não negativo. Abertura só quando saldo inicial > 0.'),
        dict(actor='Funcionário',command='Repor ou ajustar saldo',aggregate='Item de estoque',event='Movimento de entrada\nou ajuste registrado',policy='Quantidade com sinal e motivo. Saldo e movimento na mesma transação.'),
        dict(actor='Funcionário / caso de uso',command='Iniciar primeiro serviço\ne baixar peças',aggregate='OS / Item de estoque',event='Peças baixadas;\nsaídas registradas',policy='Orçamento aprovado. Bloquear itens em ordem estável; baixa única por OS.'),
        dict(actor='Funcionário',command='Consultar saldo\ne movimentos',aggregate='Item de estoque',event='Saldo e histórico\ndisponíveis',policy='Cada saída referencia sua OS. Consultar não altera saldo.',query=True),
    ]
    return [
        es('Criação da ordem de serviço','Do cadastro à publicação do orçamento. Fluxo principal preparado pelo funcionário.','event-storming-criacao',creation,'1 / 3','Os eventos descrevem fatos do negócio; os comandos são executados de forma síncrona.'),
        es('Acompanhamento e execução','Consulta, decisão do cliente e avanço da OS até a entrega.','event-storming-acompanhamento',tracking,'2 / 3','Revisar antes da execução exige novo orçamento e nova aprovação. Aprovação não reserva estoque.'),
        es('Gestão de peças e insumos','Movimentos explícitos e baixa atômica vinculada à ordem de serviço.','event-storming-estoque',stock,'3 / 3','Saldo insuficiente no início do serviço: rollback de toda a operação, sem baixa parcial.'),
    ]


MERMAID = {
'c4-n1-contexto': '''flowchart LR
    F["Funcionário\nPessoa"] -->|"Cadastros, orçamento e execução"| S["Sistema da oficina\nAPI monolítica"]
    C["Cliente\nPessoa"] -->|"Consulta da OS e decisão do orçamento"| S
    classDef person fill:#38556E,color:#fff,stroke:#38556E;
    classDef system fill:#1D5D8F,color:#fff,stroke:#1D5D8F;
    class F,C person;
    class S system;
''',
'c4-n2-containers': '''flowchart LR
    U["Consumidor HTTP ou Swagger\nFuncionário / Cliente"]
    subgraph S["Sistema da oficina"]
      A["API Go / Gin\nREST, /docs e /openapi.yaml"]
      D[("PostgreSQL 18\nJSONB, movimentos e restrições")]
      A -->|"SQL / pgx / transações"| D
    end
    U -->|"HTTP / JSON / Bearer"| A
    U -.->|"HTTPS: assets Swagger 5.33.1"| CDN["jsDelivr\nSistema externo"]
    classDef app fill:#1D5D8F,color:#fff,stroke:#1D5D8F;
    classDef ext fill:#E7E9ED,color:#172C3F,stroke:#BCC4CC;
    class A app;
    class U,CDN ext;
''',
'c4-n3-primeiro-servico': '''flowchart TB
    F["Funcionário: JWT + expectedVersion"] --> E
    subgraph API["API Go / Gin: início do primeiro serviço"]
      E["Entrada e autorização\nserver.New / auth.Admin"] --> H["Handler\nworkorders.command"]
      H --> A["Caso de uso\nworkorders.Service.Execute"]
      A -->|"Versão, aprovação e estado"| D["Domínio\nOrder.CheckVersion / StartService"]
      A --> R["Persistência da OS\nworkorders.Load / Save"]
      A --> B["Baixa dos itens\nconsume / platform.Get e Save\ndomain.AdjustStock"]
      A --> M["Movimentos\nregistry.InsertMovement"]
    end
    R -->|"OS: SELECT FOR UPDATE / gravação"| DB[("PostgreSQL 18")]
    B -->|"Itens: bloqueio em ordem estável / gravação"| DB
    M -->|"INSERT vinculado à OS"| DB
    T["Mesma transação pgx.\nFalha: rollback. Sucesso: commit."] -.-> DB
    classDef component fill:#E4F0FB,color:#172C3F,stroke:#91AFC6;
    class E,H,A,D,R,B,M component;
''',
}


def es_source(kind, stages):
    lines=['flowchart LR']
    for n,stage in enumerate(stages,1):
        actor,command,aggregate,event,policy,is_query=stage
        lines.extend([
            f'    A{n}["Ator: {actor}"]:::actor --> C{n}["{command}"]:::{"read" if is_query else "command"}',
            f'    C{n} --> G{n}["Agregado: {aggregate}"]:::aggregate',
            f'    G{n} --> E{n}["{event}"]:::{"read" if is_query else "event"}',
            f'    P{n}["Regra: {policy}"]:::policy -.-> C{n}',
        ])
        if n>1:
            lines.append(f'    E{n-1} --> C{n}')
    for name,(fill,ink) in COLORS.items():
        lines.append(f'    classDef {name} fill:{fill},color:#172C3F,stroke:{ink};')
    return '\n'.join(lines)+'\n'


def write_mermaid():
    MERMAID['event-storming-criacao']=es_source('creation',[
        ('Funcionário','Cadastrar cliente e veículo','Cliente / Veículo','Cliente e veículo cadastrados','Documento e placa únicos; vínculo com cliente',False),
        ('Funcionário','Abrir OS','OS','OS recebida','Cadastros ativos e vinculados; snapshots',False),
        ('Funcionário','Iniciar diagnóstico','OS','Diagnóstico iniciado','RECEIVED para IN_DIAGNOSIS',False),
        ('Funcionário','Definir serviços e peças','OS','Itens da OS definidos','Só RECEIVED ou IN_DIAGNOSIS; itens únicos e quantidades positivas',False),
        ('Funcionário','Publicar orçamento','OS','Orçamento publicado','Exige serviço; congela preços e total',False),
    ])
    MERMAID['event-storming-acompanhamento']=es_source('tracking',[
        ('Cliente autorizado','Consultar OS','OS','Visão restrita da OS','Token de escopo único; leitura sem transição',True),
        ('Cliente autorizado','Aprovar ou rejeitar orçamento','OS','Orçamento aprovado ou rejeitado','Só vigente e pendente; rejeição retorna ao diagnóstico; aprovação aguarda início',False),
        ('Funcionário','Iniciar serviço','OS / Estoque','Serviço e execução iniciados','Exige aprovação; primeiro início baixa peças',False),
        ('Funcionário','Concluir serviços e finalizar OS','OS','Serviços concluídos; OS finalizada','Finalizar exige todos concluídos',False),
        ('Funcionário','Registrar entrega','OS','OS entregue','Somente OS finalizada',False),
    ])
    MERMAID['event-storming-estoque']=es_source('stock',[
        ('Funcionário','Cadastrar item','Item de estoque','Item cadastrado; abertura registrada se saldo inicial positivo','SKU único; saldo não negativo',False),
        ('Funcionário','Repor ou ajustar saldo','Item de estoque','Entrada ou ajuste registrado','Quantidade assinada e motivo; mesma transação',False),
        ('Funcionário / caso de uso','Iniciar primeiro serviço e baixar peças','OS / Item de estoque','Peças baixadas; saídas registradas','Aprovado; bloqueios ordenados; baixa única; rollback se insuficiente',False),
        ('Funcionário','Consultar saldo e movimentos','Item de estoque','Saldo e histórico disponíveis','Leitura sem alterar saldo; saída referencia OS',True),
    ])
    for stem,source in MERMAID.items():
        (OUT/(stem+'.mmd')).write_text(source,encoding='utf-8')


def make_pdf(name, title, paths):
    path=OUT/name
    pdf=canvas.Canvas(str(path),pagesize=(1280,720))
    pdf.setTitle(title)
    pdf.setAuthor('Lucas Velôso - Grupo 1 - RM 377934')
    for image_path in paths:
        pdf.drawImage(ImageReader(str(image_path)),0,0,width=1280,height=720)
        pdf.showPage()
    pdf.save()
    reader=PdfReader(str(path))
    assert len(reader.pages)==3
    return path


if __name__=='__main__':
    OUT.mkdir(parents=True,exist_ok=True)
    c4=[c4_context(),c4_containers(),c4_components()]
    events=event_diagrams()
    write_mermaid()
    pdfs=[make_pdf('c4.pdf','C4 - Oficina - Níveis 1, 2 e 3',c4),make_pdf('event-storming.pdf','Event Storming - Oficina',events)]
    for path in c4+events+pdfs:
        print(path)
