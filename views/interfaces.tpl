
<div class="p-3 row">

    <h3>Here is a list of your interfaces</h3>
    <ul class="list-group">
    {{range .Interfaces}}
        <li class="list-group-item">Idx: {{ .Index }}, Name: {{ .Name }}</li>
    {{end}}
    {{range .LinkList}}
        <li class="list-group-item">Link: {{ .Type }} {{ .Attrs.Name }} </li>
    {{end}}
    </ul>

</div>